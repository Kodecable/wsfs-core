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

For more information about configuration files, see [server-config-exmaple.toml](https://github.com/Kodecable/wsfs-core/blob/main/doc/server-config-exmaple.toml).

For running the server as a systemd service, see [wsfs-systemd-service-exmaple.service](https://github.com/Kodecable/wsfs-core/blob/main/doc/server-config-exmaple.service).

### Quick Serve Command

This command starts a server using the provided options. To view all available options and examples:

```shell
$ wsfs quick-serve --help
```

This command is essentially a wrapper around the Serve Command.

WebDAV and WebUI are enabled, with WebUI custom disabled.

If no username is given, anonymous access is enabled. If no password is provided, a random one will be generated and printed. If no storage path is specified, the server will use the working directory.

Servers started by this command cannot be reloaded.

### Reload Command

This command instructs the server to reload its configuration. To view all available options:

```shell
$ wsfs reload-server --help
```

This command is only available on Linux.

If SSL certificates or WebUI custom resources have changed, please reload the server to ensure it works properly.

If no PID is specified, it will automatically try to find one. However, this auto-find function is not recommended for production use.

This command cannot check the result of reload; please check the server's log.

### Hash Command

This command generates a bcrypt hash used in server configuration. To view all available options:

```shell
$ wsfs hash --help
```

If no password is given as arguments, one will be read from stdin.

## Client

**Warning:** Client function has been tested only on Linux and Windows.

### Mount

To mount a WSFS, you must satisfy the extra requirements outlined in [installation.md](https://github.com/Kodecable/wsfs-core/blob/main/doc/installation.md).

To view all available options and examples:

```shell
$ wsfs mount --help
```

#### Linux

Although FUSE supports multi-user access, it is very dangerous and not recommended on WSFS. We recommend running WSFS mount as a normal user and only using this user to access the file system.

If you are on Android, you may need root to mount, use the option `--direct-mount`.

#### Windows

Windows will not accept special characters (e.g., "<") in filenames. Please avoid using them.

Most permissions will be interpreted as file modes. Please avoid manipulating permissions. Just read and write files normally.
