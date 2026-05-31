# Mount Blackbox Tests

These tests exercise the real mount path end to end:

1. Start `wsfs quick-serve`
2. Start `wsfs mount`
3. Operate on the mounted filesystem
4. Verify both mount-visible results and backend storage state

The goal is not to replace unit tests. The goal is to catch real mount
semantics regressions after changes to protocol handling, FUSE nodes, caching,
or platform-specific mount code.

## Requirements

### Linux

- FUSE available to the current user
- A built `wsfs` binary

### Windows

- WinFsp installed
- A Windows environment where drive-letter mounts are allowed
- A built `wsfs` binary

The current repository has the Windows harness adapter structure, but it still
needs real Windows execution to validate runtime behavior and adjust any
platform-specific expectations.

## Usage

```bash
go run . --help
```

## Work Directory

The default work directory is:

```text
../../run/mount-blackbox/
```

Each case creates:

- `backend/`
- `mount/` on Unix, or a platform-specific mount target on Windows
- `logs/server.log`
- `logs/mount.log`

The harness starts `quick-serve` and `mount` with `--no-log-color` so captured logs
stay readable in files.

Successful cases are removed by default. Failed cases are preserved.
Skipped cases are reported separately from passed cases.

## Covered Semantics

The current suite focuses on high-value mount semantics:

- create, overwrite, append
- truncate shrink and expand
- `ReadAt` and `WriteAt`
- large file reads and writes across message boundaries
- root and nested directory visibility
- rename and remove visibility
- `getattr` after write and rename
- symlink creation, `readlink`, and reading through symlinks
- many small random writes

These cases are designed to detect regressions in:

- command dispatch and protocol framing
- response chunking
- directory and attribute cache invalidation
- symlink path handling
- platform-specific mount behavior

## Notes

- A failing blackbox case should not automatically be blamed on the test code.
  This suite has already exposed several real business logic regressions during
  the recent refactor.
- If you change mount semantics, symlink behavior, cache invalidation, or
  platform-specific client code, run this suite before committing.
