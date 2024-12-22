# Usage

[toc]

## Server

**Warning:** Server function has been tested only on Linux.

You can start a server using one of two methods: the Serve Command or the Quick Serve Command. If you plan to run the WSFS Server temporarily and don't need extensive customization, the Quick Serve Command may be helpful. Otherwise, please use the Serve Command.

### Serve Command

This command will start a server based on the provided configuration file. To view all available options:

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

The server do not support gracefully shutdown currently.

For more information about configuration files, see [server-config-exmaple.toml](https://github.com/Kodecable/wsfs-core/blob/main/doc/server-config-exmaple.toml).

For run the server as a systemd service, see [wsfs-systemd-service-exmaple.service](https://github.com/Kodecable/wsfs-core/blob/main/doc/server-config-exmaple.service).

### Quick Serve Command

This command will start a server using the provided options. To view all available options and exmaples:

```shell
$ wsfs quick-serve --help
```

This command is essentially a wrapper around the Serve Command, sharing common features such as not supporting graceful shutdowns.

WebDAV and WebUI are enabled, with WebUI custom disabled.

If no username is given, anonymous is enabled. If no password is provided, random one will be generated and printed. If no storage path is specified, the server will use the working directory.

Servers started by this command cannot be reloaded.

### Reload Command

This command instructs the server to reload its configuration. To view all available options:

```shell
$ wsfs reload-server --help
```

This command only available in Linux.

If SSL certificates or WebUI custom resources have changed, please reload the server to ensure it works fine.

If no PID is specified, it will automatically try to find one. However, this auto-find function is not recommended for production use.

The reload operation is thread-safe. If new configuration has errors, the server will refuse to reload.

This command cannot check the result of reload; please check it at the server's log.

During reload, there is a very small gap that the server will not listen on its port even if listener configuration remains unchanged. New HTTP requests to the server during this gap will fail. If another program takes the port during this gap, the server will exit with an error.

### Hash command

This command generates a bcrypt hash used in server configuration. To view all available options:

```shell
$ wsfs hash --help
```

If no password is given as arguments, one will be read from stdin.

## Client

**Warning:** Client function has been tested only on Linux and Windows.

### Mount

To mount a WSFS, you must satisfy the extra requirements outlined in [installation.md](https://github.com/Kodecable/wsfs-core/blob/main/doc/installation.md).

To view all available options and exmaples:

```shell
$ wsfs mount --help
```

#### Linux

Although FUSE supports multi-user access, it is very dangerous and not recommended on WSFS. We recommend running WSFS mount as a normal user and only using this user to access the file system.

If you are in Android, you need root to mount. Use options `--direct-mount`.

#### Windows

Windows will not accept special characters (e.g., "<") in filenames. Please avoid using them.

Most permissions will be interpreted to file modes. Please avoid playing with permissions. Just read and write.
