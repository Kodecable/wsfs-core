# Build Guide

## Requirements

- **Bash**: Required for build scripts
- **Go Toolchain**: check `src/go.mod` for minimum required version
- **Optional**: [minify](https://github.com/tdewolff/minify) for release builds

## Building from Source

### Quick Build

```bash
# Build with default settings (debug mode, all platforms and architectures)
./build.sh
```

### Build Options

```bash
# Show help
./build.sh -h

# Clean build directory
./build.sh -c

# Build in release mode (optimized, smaller binaries)
./build.sh -m release

# Build for specific OS
./build.sh -o linux
./build.sh -o windows

# Build for specific architecture
./build.sh -a amd64
./build.sh -a arm64

# Build with specific version
./build.sh -v "1.0.0"
```

### Supported Platforms and Architectures

**Linux:**
- 386, amd64, amd64v3, arm, arm64

**Windows:**
- 386, amd64, amd64v3, arm64

## Output

The compiled binaries will be located in the `build` directory with naming pattern:
- `wsfs-{os}-{arch}` (e.g., `wsfs-linux-amd64`, `wsfs-windows-amd64.exe`)

