# Usage

[toc]

## Server

**Warning:** Server function has been tested only on Linux.

You can start a server using one of two methods: the Serve Command or the Quick Serve Command. If you plan to run the WSFS Server temporarily and don't need extensive customization, the Quick Serve Command may be helpful. Otherwise, please use the Serve Command.

### Serve Command

This command starts a server based on the provided configuration file. To view all available options:

```shell
$ wsfs serve --help
```

If you don't specify a config file option, the server will search for the following files in order:

1. `./server.toml`
2. `./wsfs.toml`
3. `./config.toml`
4. `~/.config/wsfs/server.toml`
5. `/etc/wsfs/server.toml`

If no configuration file is found or if the specified path doesn't exist, the server will start with the default settings. In production, we recommend against relying on automatic config file search or the default settings.

For more information about configuration files, see [server-config-example.toml](https://github.com/Kodecable/wsfs-core/blob/main/doc/server-config-example.toml).

For running the server as a systemd service, see [wsfs-systemd-service-example.service](https://github.com/Kodecable/wsfs-core/blob/main/doc/server-config-example.service).

### Quick Serve Command

This command starts a server using the provided options. To view all available options and examples:

```shell
$ wsfs quick-serve --help
```

This command is essentially a wrapper around the Serve Command.

WebDAV and WebUI are enabled, with WebUI custom disabled.

If no username is given, writable anonymous access is enabled. Do not expose a server started in this mode to an untrusted network. If a username is given but no password is provided, a random password will be generated and printed. If no storage path is specified, the server will use the working directory.

Servers started by this command cannot be reloaded.

### Signal Handling

On Unix systems, the `serve` command handles the following signals:

- `SIGHUP` reloads the server configuration.
- `SIGINT` and `SIGTERM` request a graceful shutdown. The server stops accepting new HTTP connections and waits for active HTTP requests to finish. It does not explicitly close long-lived WSFS sessions in the shutdown handler.

The `quick-serve` command handles all the three signals as a graceful shutdown.

### Reload Command

This command instructs the server to reload its configuration. To view all available options:

```shell
$ wsfs reload-server --help
```

This command is only available on Linux.

If SSL certificates or WebUI custom resources have changed, please reload the server to ensure it works properly.

If no PID is specified, it will automatically try to find one. However, this auto-find function is not recommended for production use.

This command cannot check the result of reload; please check the server's log.

Reloading does not intentionally destroy established WSFS sessions. For listener changes, the server starts the replacement listener before shutting down the old one and waits for active work to finish. See [technical.md](https://github.com/Kodecable/wsfs-core/blob/main/doc/technical.md) for the session lifecycle details.

### Hash Command

This command generates a bcrypt hash used in server configuration. To view all available options:

```shell
$ wsfs hash --help
```

If no password is given as arguments, one will be read from stdin.

### Password Sources

The `--password` option is available for `mount` and `quick-serve`. It accepts one of the following sources:

- `stdin` reads the password interactively or from standard input.
- `env:NAME` reads the password from the environment variable `NAME`.
- `file:PATH` reads the password from `PATH` and removes trailing line endings.

For `mount`, the URL must include a username when `--password` is used. A password embedded in the URL and `--password` cannot be used together. Using a password source is recommended when putting credentials in the command line or shell history would be undesirable.

## Client

**Warning:** Client function has been tested only on Linux and Windows.

### Mount

To mount a WSFS, you must satisfy the extra requirements outlined in [installation.md](https://github.com/Kodecable/wsfs-core/blob/main/doc/installation.md).

To view all available options and examples:

```shell
$ wsfs mount --help
```

When connecting over TLS (`wsfss://` or `wsfss+unix://`), you can use `--cert-hash` to pin the server certificate by hash.

When `--cert-hash` is set, WSFS skips the normal TLS certificate verification and only checks whether the server leaf certificate hash exactly matches the provided value.

WSFS always prints the server certificate hash during TLS connection attempts so operators can copy it directly from the log output instead of formatting it manually.

Typical workflow:

1. Connect once without `--cert-hash` and note the `Server cert received` log line.
2. Copy the printed hash value, for example `SHA256:0123456789abcdef...`.
3. Re-run mount with `--cert-hash <copied-hash>`.

The client sends WebSocket ping frames every 60 seconds by default. Use `--ping-interval 0` to disable client keepalive, or set another interval of at least 10 seconds. A failed ping triggers the session recovery process described in [technical.md](https://github.com/Kodecable/wsfs-core/blob/main/doc/technical.md).

#### Linux

Although FUSE supports multi-user access, it is very dangerous and not recommended on WSFS. We recommend running WSFS mount as a normal user and only using this user to access the file system.

The mount client supports `SIGHUP`, `SIGINT`, and `SIGTERM` for graceful shutdown. When it receives one of these signals, it waits for all ongoing requests to finish and completes the WebSocket close handshake before shutting down.

On Android, where the userspace FUSE mounting tools may be unavailable, use `--direct-mount`. This option uses the `mount` syscall and requires root privileges. For details, see [technical.md](https://github.com/Kodecable/wsfs-core/blob/main/doc/technical.md).

The `--flock` option controls how BSD `flock(2)` requests are handled. For details, see [technical.md](https://github.com/Kodecable/wsfs-core/blob/main/doc/technical.md).

When the WebSocket connection is interrupted, the client attempts to resume the existing WSFS session automatically. The session may be unavailable briefly while the server finishes handling the previous connection.

#### Windows

Windows will not accept special characters (e.g., "<") in filenames. Please avoid using them.

Most permissions will be interpreted as file modes. Please avoid manipulating permissions. Just read and write files normally.
