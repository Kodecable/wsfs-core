# Technical

[toc]

## Server

### WebDAV

This server supports full WebDAV Class 1 without properties and locks. It ignores lock conditions and does not support the `LOCK`/`UNLOCK` methods. It responds with limited properties (`resourcetype`, `getcontenttype`, `getcontentlength`, `displayname`, `getlastmodified`) and ignores the `PROPPATCH` method.

This server supports the `PATCH` method for [sabre/dav partial update](https://sabre.io/dav/http-patch/).

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

During reload, there is a very small gap where the server will not listen on its port even if the listener configuration remains unchanged. New HTTP requests to the server during this gap will fail. If another program takes the port during this gap, the server will exit with an error.

### WebUI

The WebUI is designed for modern browsers. It requires no cookies. JavaScript is optional; without it, you can still view a directory index, but cannot perform uploads or other interactive operations.

### Reverse Proxy

This server will automatically detect the `Host` header (in fact, most code avoids using the host/base-url). You only need to forward connections to it, and it should handle them perfectly.

By default, the server uses the direct connection address as the client address. If you run behind a reverse proxy and need the upstream client IP in logs or something, set `RealIpHeader` in server config. When `RealIpHeader` is set, the server uses the first element in that header as the remote address.

```toml
RealIpHeader = "X-Forwarded-For"
# Common choices are `X-Forwarded-For` or `X-Real-IP`, depending on your proxy setup.
```

## Client

### Mount

In Linux, including Android, this client supports two ways to mount the file system:
- Using syscall `mount`
- Using userspace tools `fusemount3` / `fusemount`

The first method requires root privileges but does not need userspace tools.
