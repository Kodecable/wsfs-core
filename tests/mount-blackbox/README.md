# Mount Blackbox Tests

These tests exercise the real mount path end to end:

1. Start or connect to a WSFS server
2. Start `wsfs mount`
3. Prepare test data through the mounted filesystem
4. Operate on the mounted filesystem
5. Verify mount-visible results
6. Verify backend storage state when the storage root is visible to the runner

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

By default the runner starts both sides for each case:

```bash
go run . --wsfs-bin ../../build/wsfs-linux-amd64
```

To test only the mount client against an already-running server, pass an
endpoint and a relative test directory. In this mode the runner does not start
`quick-serve`; before each case it removes and recreates the test directory
through the mount, and all case operations stay under that directory:

```bash
go run . --wsfs-bin ../../build/wsfs-linux-amd64 --endpoint wsfs://127.0.0.1:20001/ --test-dir wsfs-blackbox
```

Mount-visible verification is always performed through the mounted filesystem.
When the runner starts `quick-serve` itself, backend storage verification is
performed automatically. For an external endpoint, pass the server storage root
when that directory is visible to the test runner; storage verification is then
performed against `--storage-dir/--test-dir`:

```bash
go run . --wsfs-bin ../../build/wsfs-linux-amd64 --endpoint wsfs://127.0.0.1:20001/ --test-dir wsfs-blackbox --storage-dir /srv/wsfs-test
```

## Work Directory

The default work directory is:

```text
../../run/mount-blackbox/
```

Each case creates:

- `backend/` when the runner starts its own server
- `mount/` on Unix, or a platform-specific mount target on Windows
- `logs/server.log`
- `logs/mount.log`

When `--endpoint` is not set, the harness starts `quick-serve` and `mount` with
`--no-log-color` so captured logs stay readable in files. When `--endpoint` is
set, only `mount` is started.

Successful cases are removed by default. Failed cases are preserved.
Skipped cases are reported separately from passed cases.

## Covered Semantics

The current suite focuses on high-value mount semantics:

- create, overwrite, append
- truncate shrink and expand
- `ReadAt` and `WriteAt`
- large file reads and writes across message boundaries
- interleaved random reads and writes across 8 x 64 MiB files
- root and nested directory visibility
- empty child directory visibility after parent readdir prefetch
- rename and remove visibility
- `getattr` after write and rename
- symlink creation, `readlink`, and reading through symlinks
- many small random writes
- random directory walks across 6-level deep trees with 16 to 1600 entries per level

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
