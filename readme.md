# WSFS-Core

**Warning:** This project is currently **WIP** and may contains numerous **bugs**.

WSFS-Core is an implementation of WSFS which designed to provide a lightweight, nearly complete remote-mounting experience.

Written in pure Go, WSFS-Core can serve WSFS on almost all Go-supported operating systems and architectures. It can also mounting WSFS on Windows, Linux (including Android) and Darwin. (Currently, It is recommended to run WSFS-Core on Linux.)

WSFS-Core can also serve a limited WebDAV and a simple WebUI when serve WSFS.

## Build

You can use the build script on most UNIX-like systems.

```shell
$ ./build.sh -h
```

The output will be located in the `Build` directory.

## Install

WSFS-Core is a single executable file, which is all you need.

Pre-built binaries and packages are also available.

## Usage

You can learn command usage by:

```shell
$ wsfs --help
```

Server config file exmaple is also [available](https://github.com/Kodecable/wsfs-core/blob/main/doc/server-config-exmaple.toml).

In android, you may need root and run command like this to mount WSFS.

```shell
$ wsfs mount "wsfs://USERNAME:PASSWORD@HOST:PORT/?wsfs" MOUNTPOINT --uid 0 --gid 0 --nobody-uid 9999 --nobody-gid 9999 --direct-mount
```

## License

MIT
