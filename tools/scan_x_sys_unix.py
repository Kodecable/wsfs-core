#!/usr/bin/env python3

import glob
import os
import re


BASE = os.environ.get("X_SYS_UNIX_DIR")
if not BASE:
    gopath = os.environ.get("GOPATH", os.path.expanduser("~/go"))
    BASE = os.path.join(gopath, "pkg/mod/golang.org/x/sys@v0.45.0/unix")
print(f"BASE: {BASE}")

FAMILIES = {
    "open": [
        "O_RDONLY",
        "O_WRONLY",
        "O_RDWR",
        "O_TRUNC",
        "O_EXCL",
        "O_CREAT",
        "O_DIRECTORY",
        "O_APPEND",
        "O_NONBLOCK",
        "O_DSYNC",
        "O_SYNC",
        "O_DIRECT",
        "O_LARGEFILE",
        "O_NOFOLLOW",
        "O_NOATIME",
        "O_CLOEXEC",
        "O_PATH",
        "O_TMPFILE",
        "O_NOCTTY",
        "O_ASYNC",
    ],
    "seek": ["SEEK_SET", "SEEK_CUR", "SEEK_END", "SEEK_DATA", "SEEK_HOLE"],
    "lock": ["F_RDLCK", "F_UNLCK", "F_WRLCK"],
    "ofdlock": ["F_OFD_SETLK", "F_OFD_SETLKW", "F_OFD_GETLK"],
    "rename": ["RENAME_NOREPLACE", "RENAME_EXCHANGE", "RENAME_WHITEOUT"],
    "falloc": [
        "FALLOC_FL_KEEP_SIZE",
        "FALLOC_FL_PUNCH_HOLE",
        "FALLOC_FL_COLLAPSE_RANGE",
        "FALLOC_FL_ZERO_RANGE",
        "FALLOC_FL_INSERT_RANGE",
        "FALLOC_FL_UNSHARE_RANGE",
    ],
}

FUNCTIONS = [
    "CopyFileRange",
    "Fallocate",
    "Renameat2",
]

PATTERN = re.compile(r"\b([A-Z0-9_]+)\b\s*=\s*([^\s]+)")
FUNC_PATTERN = re.compile(r"^func\s+([A-Za-z_][A-Za-z0-9_]*)\b", re.MULTILINE)
SYS_PATTERN = re.compile(r"^//sys(?:nb)?\s+([A-Za-z_][A-Za-z0-9_]*)\b", re.MULTILINE)
BUILD_PATTERN = re.compile(r"^//go:build\s+(.+)$", re.MULTILINE)

GOOSES = {
    "aix",
    "android",
    "darwin",
    "dragonfly",
    "freebsd",
    "hurd",
    "illumos",
    "ios",
    "linux",
    "netbsd",
    "openbsd",
    "solaris",
    "zos",
}

GOARCHES = {
    "386",
    "amd64",
    "arm",
    "arm64",
    "loong64",
    "mips",
    "mips64",
    "mips64le",
    "mipsle",
    "ppc",
    "ppc64",
    "ppc64le",
    "riscv64",
    "s390x",
    "sparc64",
    "wasm",
}


def collect_rows():
    rows = []
    for path in glob.glob(os.path.join(BASE, "zerrors_*.go")):
        target = os.path.basename(path)[8:-3]
        values = {}
        with open(path, "r", encoding="utf-8", errors="ignore") as handle:
            for line in handle:
                match = PATTERN.search(line)
                if match:
                    values[match.group(1)] = match.group(2)
        rows.append((target, values))
    return rows


def split_target(target):
    parts = target.split("_")
    goos = parts[0]
    goarch = parts[1] if len(parts) > 1 else None
    return goos, goarch


class BuildExpr:
    def __init__(self, tokens, tags):
        self.tokens = tokens
        self.tags = tags
        self.pos = 0

    def parse(self):
        value = self.parse_or()
        if self.pos != len(self.tokens):
            raise ValueError("trailing tokens")
        return value

    def parse_or(self):
        value = self.parse_and()
        while self.match("||"):
            value = value or self.parse_and()
        return value

    def parse_and(self):
        value = self.parse_not()
        while self.match("&&"):
            value = value and self.parse_not()
        return value

    def parse_not(self):
        if self.match("!"):
            return not self.parse_not()
        return self.parse_atom()

    def parse_atom(self):
        if self.match("("):
            value = self.parse_or()
            if not self.match(")"):
                raise ValueError("missing closing paren")
            return value
        token = self.next()
        return token in self.tags

    def match(self, token):
        if self.pos < len(self.tokens) and self.tokens[self.pos] == token:
            self.pos += 1
            return True
        return False

    def next(self):
        if self.pos >= len(self.tokens):
            raise ValueError("unexpected end")
        token = self.tokens[self.pos]
        self.pos += 1
        return token


def eval_build_expr(expr, tags):
    tokens = re.findall(r"\|\||&&|!|\(|\)|[A-Za-z0-9_.]+", expr)
    return BuildExpr(tokens, tags).parse()


def filename_tags(path):
    base = os.path.basename(path)
    stem = base[:-3]
    if stem.endswith("_test"):
        stem = stem[:-5]
    parts = stem.split("_")
    tags = []
    for part in parts[1:]:
        if part in GOOSES or part in GOARCHES:
            tags.append(part)
        elif part in {"gc", "gccgo", "libc"}:
            tags.append(part)
    return tags


def file_matches_target(path, target):
    goos, goarch = split_target(target)
    tags = {goos, "unix", "gc"}
    if goarch:
        tags.add(goarch)
    for tag in filename_tags(path):
        if tag in GOOSES and tag != goos:
            return False
        if tag in GOARCHES and tag != goarch:
            return False
    with open(path, "r", encoding="utf-8", errors="ignore") as handle:
        text = handle.read()
    match = BUILD_PATTERN.search(text)
    if match:
        try:
            return eval_build_expr(match.group(1), tags)
        except ValueError:
            return False
    return True


def collect_functions(rows):
    targets = [target for target, _ in rows]
    functions = {target: set() for target in targets}
    sources = {target: {} for target in targets}
    paths = [
        path
        for path in glob.glob(os.path.join(BASE, "*.go"))
        if not os.path.basename(path).endswith("_test.go")
    ]
    for path in paths:
        with open(path, "r", encoding="utf-8", errors="ignore") as handle:
            text = handle.read()
        names = set(FUNC_PATTERN.findall(text))
        names.update(SYS_PATTERN.findall(text))
        if not names:
            continue
        for target in targets:
            if not file_matches_target(path, target):
                continue
            for name in names:
                functions[target].add(name)
                sources[target].setdefault(name, []).append(os.path.basename(path))
    return functions, sources


def print_function_report(rows, names):
    functions, sources = collect_functions(rows)
    for name in names:
        present = []
        missing = []
        for target, _ in rows:
            if name in functions[target]:
                present.append(target)
            else:
                missing.append(target)
        print(f"## func {name}")
        print(f"PRESENT: {len(present)} targets")
        for target in present:
            print(f"  {target}: {', '.join(sorted(sources[target][name]))}")
        print(f"MISSING: {len(missing)} targets")
        if missing:
            print("  " + ", ".join(missing))
        print()


def print_constant_report(rows):
    for family, names in FAMILIES.items():
        groups = {}
        for target, values in rows:
            present = tuple(name for name in names if name in values)
            if present:
                groups.setdefault(present, []).append(target)
        print(f"## {family}")
        for index, (present, targets) in enumerate(sorted(groups.items(), key=lambda item: (len(item[0]), item[1][0]))):
            print(f"GROUP {index + 1}: {len(targets)} targets")
            print("  " + ", ".join(present))
            print("  sample: " + ", ".join(targets))
        print()


def main():
    rows = collect_rows()
    print_constant_report(rows)
    print_function_report(rows, FUNCTIONS)


if __name__ == "__main__":
    main()
