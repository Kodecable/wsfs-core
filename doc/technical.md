# Technical

[toc]

## Server

### WebDAV

This server supports full WebDAV Class 1 without properties and locks. It ignores lock conditions and does not support the `LOCK`/`UNLOCK` methods. It responds with limited properties (`resourcetype`, `getcontenttype`, `getcontentlength`, `displayname`, `getlastmodified`) and ignores the `PROPPATCH` method.

This server supports the `PATCH` method for [sabre/dav partial update](https://sabre.io/dav/http-patch/).

#### Download Query

For `GET` and `HEAD` requests, the `download` query parameter controls the `Content-Disposition` response header. When the parameter is present, the response is sent as an attachment unless its value is `false` or `0`:

- `/path/to/file?download` and `/path/to/file?download=1` return an attachment.
- `/path/to/file?download=false` and `/path/to/file?download=0` return an inline response.

#### Root Quota Properties

A `PROPFIND` request for the storage root includes `quota-available-bytes` and `quota-used-bytes` when the underlying filesystem provides capacity information. These values describe the underlying filesystem; they are not an enforced WSFS quota. The properties are reported on the root only.

#### Test

This server passed all tests in [litmus](http://www.webdav.org/neon/litmus/) 0.13 `basic` and `copymove`:

```
-> running `basic':
 0. init.................. pass
 1. begin................. pass
 2. options............... WARNING: server does not claim Class 2 compliance
    ...................... pass (with 1 warning)
 3. put_get............... pass
 4. put_get_utf8_segment.. pass
 5. put_no_parent......... pass
 6. mkcol_over_plain...... pass
 7. delete................ pass
 8. delete_null........... pass
 9. delete_fragment....... pass
10. mkcol................. pass
11. mkcol_again........... pass
12. delete_coll........... pass
13. mkcol_no_parent....... pass
14. mkcol_with_body....... pass
15. finish................ pass
<- summary for `basic': of 16 tests run: 16 passed, 0 failed. 100.0%
-> 1 warning was issued.
-> running `copymove':
 0. init.................. pass
 1. begin................. pass
 2. copy_init............. pass
 3. copy_simple........... pass
 4. copy_overwrite........ pass
 5. copy_nodestcoll....... pass
 6. copy_cleanup.......... pass
 7. copy_coll............. pass
 8. copy_shallow.......... pass
 9. move.................. pass
10. move_coll............. pass
11. move_cleanup.......... pass
12. finish................ pass
<- summary for `copymove': of 13 tests run: 13 passed, 0 failed. 100.0%
```

### Reload

Servers that support reload will handle the POSIX signal `SIGHUP` and reload their configuration.

The reload operation is thread-safe. If the new configuration has errors, the server will refuse to reload and continue using the old configuration.

The WSFS session registry is retained across reloads. When the listener is replaced, the old HTTP server is shut down after the new listener is ready, so long-lived WSFS sessions can survive a listener reload. The server waits for in-flight requests and does not impose a deadline on this shutdown wait.

### WebUI

The WebUI is designed for modern browsers. It requires no cookies. JavaScript is optional; without it, you can still view a directory index, but cannot perform uploads or other interactive operations.

### Reverse Proxy

This server will automatically detect the `Host` header (in fact, most code avoids using the host/base-url). You only need to forward connections to it, and it should handle them perfectly.

By default, the server uses the direct connection address as the client address. If you run behind a reverse proxy and need the upstream client IP in logs or something, set `RealIpHeader` in server config. When `RealIpHeader` is set, the server uses the first element in that header as the remote address.

```toml
RealIpHeader = "X-Forwarded-For"
# Common choices are `X-Forwarded-For` or `X-Real-IP`, depending on your proxy setup.
```

### WSFS

#### Hard Links

Hard-link operations are implemented only on the server side.

WSFS does not use an inode model, and does not support the hard-link count (`st_nlink`). Hard links are also a poor fit because the server may span multiple filesystems while the client sees only a single WSFS filesystem. Symbolic links are recommended instead.

#### XAttr Filtering

WSFS natively supports Linux-style extended attributes (xattrs), but the server filters xattr operations by key prefix. By default, no prefixes are allowed, so all xattrs are blocked. Blocked xattrs are omitted from list results, and operations targeting blocked keys are rejected.

## Client

### Mount

#### Linux

##### Direct Mount

In Linux, including Android, this client supports two ways to mount the file system:
- Using the `mount` syscall. (`--direct-mount`)
- Using the userspace tools `fusemount3` / `fusemount` (the default)

The direct-mount method requires root privileges but does not require userspace FUSE tools. It can be useful on Android, where those tools may be unavailable.

##### FUSE Connection ID

On Linux, when the mount uses the default userspace `fusemount` method, the mount client tries to read and log the FUSE connection ID from `/proc/self/mountinfo`. This value also can be found as the Dev field in the Stat_t result for a file in the mount. If the ID cannot be determined, the client logs a warning instead.

The connection ID can be used to abort a FUSE connection when a mount is stuck and cannot be unmounted normally. Replace `<id>` and `<mountpoint>` with the values from the mount log:

```shell
$ sudo sh -c 'echo 1 > /sys/fs/fuse/connections/<id>/abort'
$ fusermount3 -uz <mountpoint>
```

Aborting a connection is an emergency recovery procedure. In-flight FUSE requests will fail, so use it only after a normal unmount cannot complete.

##### BSD Flock

WSFS natively supports OFD locks but does not support BSD `flock(2)` locks. The mount client provides three handling modes:

- `ofd` maps `flock` to whole-file OFD locks. This is the default. It provides practical cross-client locking, but `flock` shares the same conflict domain as WSFS fcntl/OFD byte-range locks and is not a strict Linux local-filesystem `flock` emulation.
- `unsupported` returns `ENOTSUP` for `flock` requests.
- `noop` reports `flock` success without taking any lock.

##### XAttr Filtering

The mount client applies an independent xattr filter. The client and server filters are independent, so a key must pass both filters. See the Server section above for more information about server-side xattr filtering.

For xattr values that do not fit in one WSFS command, the client splits a write into an initial set operation followed by append operations by default. This can expose an intermediate or partially updated value, so atomic all-or-nothing behavior cannot be guaranteed for oversized xattr writes.

Disable XAttr Append when an oversized xattr write must fail instead of being split. In that mode, the client returns `ERANGE` before sending the write. This prevents the client's append sequence but does not change the atomicity guarantees of the underlying filesystem for other xattr operations.

#### Unix Mode Bits

The Unix mount client preserves the setuid, setgid, and sticky bits when translating file modes between the local mount interface and WSFS. The backend filesystem's normal permission rules still apply.

## Protocol

### Modification Time

The WSFS protocol transmits only `mtime`; it does not carry independent `atime` or `ctime` values. The wire representation stores seconds and nanoseconds. The mount clients use the transmitted `mtime` for the local file timestamps, so independent access and change times cannot be preserved across WSFS.

On Linux 32-bit systems, the server uses time64-compatible system calls where necessary to avoid the Y2038 limitation. The protocol can carry timestamps beyond 2038, subject to the capabilities of the backend filesystem and operating system.

### Request Scheduling and Splitting

For read, write, and `copy_file_range` operations, the client does not necessarily send exactly one WSFS request for each kernel operation. The server does not necessarily translate each WSFS request into exactly one syscall either; it may split or combine work as needed. The server does not guarantee the order in which concurrent syscalls start or complete, but it sends a response only after the syscall or syscalls associated with that response have completed. Other operations, such as `open` and `renameat`, preserve their operation-specific atomicity and request semantics.

Large reads will read data in segments on the server, large writes use the internal write-stream mechanism, and large `copy_file_range` calls are split into smaller requests. A single `copy_file_range` request is limited to 32 MiB; the client automatically splits larger operations.

For read, write, and `copy_file_range`, callers must rely on returned byte counts and errors, and must not assume a one-to-one mapping between kernel requests and protocol messages or completion events.

### Session Resume

After the initial WSFS WebSocket handshake, the server returns a session identifier in the `X-Wsfs-Resume` header.

The server keeps a disconnected session in a hibernated state instead of immediately discarding it. When the connection is lost, the mount client sends the `X-Wsfs-Resume` header on a new WebSocket handshake to resume the existing session. A normal WebSocket close initiated by the mount client removes the session rather than hibernating it.

### WebSocket Keepalive

The mount client sends WebSocket ping frames at a configurable interval. Set it to `0` to disable client keepalive. A ping that does not complete within 10 seconds is treated as a connection failure and triggers session recovery.
