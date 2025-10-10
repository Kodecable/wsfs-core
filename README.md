# WSFS-Core

**Warning:** This project is currently **WIP** and may contain numerous **bugs**.

WSFS-Core is an implementation of WSFS (WebSocket File System) designed to provide a lightweight, nearly complete remote-mounting experience.

Written in pure Go, WSFS-Core can serve WSFS on almost all Go-supported operating systems and architectures. It can also mount WSFS on Windows, Linux (including Android) and Darwin. (Currently, it is recommended to run WSFS-Core on Linux.)

WSFS-Core can also serve a limited WebDAV and a simple WebUI when serving WSFS.

## Quick Start

### Installation

Download pre-built binaries from the [releases](https://github.com/Kodecable/wsfs-core/releases) or build from source.

### Running a Server

```bash
# Quick start with all default
wsfs quick-serve

# Or with configuration file
wsfs serve -c server.toml
```

### Mounting a Filesystem

```bash
# Mount WSFS to local directory
wsfs mount wsfs://localhost:20001 /mnt/wsfs
```

## Documentation

- [Build Guide](doc/build.md)
- [Installation Guide](doc/installation.md)
- [Usage Guide](doc/usage.md)
- [Technical Details](doc/technical.md)
- [Configuration Example](doc/server-config-exmaple.toml)
- [Systemd Service](doc/server-config-exmaple.service)

## License

MIT License
