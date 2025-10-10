# Installation

[toc]

## Linux

### Arch Linux

A pre-built package for x86_64 is available in the [releases](https://github.com/Kodecable/wsfs-core/releases). You can download it, than install it using the following command:

```shell
$ sudo pacman -U /path/to/file.pkg.tar.zst
```

If you plan to mount WSFS, make sure to install the `fuse3` package as well:

```shell
$ sudo pacman -S --needed fuse3
```

### Debian

Pre-built packages are available in the [releases](https://github.com/Kodecable/wsfs-core/releases). You can download and install them using the following command:

```shell
$ sudo apt-get install /path/to/file.deb
```

If you plan to mount WSFS, make sure to install the `fuse3` package as well:

```shell
$ sudo apt-get install fuse3
```

### Other Distributions

You can download pre-built binaries from the [releases](https://github.com/Kodecable/wsfs-core/releases).

## Windows

WSFS-Core is distributed as a single .exe file. You can download pre-built binaries from the [releases](https://github.com/Kodecable/wsfs-core/releases).

If you plan to mount WSFS, you'll also need to install the third-party utility [WinFsp](https://winfsp.dev/).
