# Installation

## go install (Recommended)

```bash
go install github.com/lleontor705/opencode-model-selector@latest
```

Binary goes to `$GOPATH/bin/opencode-model-selector` (typically `~/go/bin/` or `%USERPROFILE%\go\bin\`).

## Build from Source

```bash
git clone https://github.com/lleontor705/opencode-model-selector.git
cd opencode-model-selector
go build -ldflags="-s -w" -o opencode-model-selector .
```

With version stamp:

```bash
go build -ldflags="-s -w -X main.version=local-$(git describe --tags --always)" -o opencode-model-selector .
```

## Pre-built Binaries

Download from [Releases](https://github.com/lleontor705/opencode-model-selector/releases):

| Platform | File |
|----------|------|
| Linux x86_64 | `opencode-model-selector_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `opencode-model-selector_<version>_linux_arm64.tar.gz` |
| macOS Intel | `opencode-model-selector_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `opencode-model-selector_<version>_darwin_arm64.tar.gz` |
| Windows x86_64 | `opencode-model-selector_<version>_windows_amd64.zip` |
| Windows ARM64 | `opencode-model-selector_<version>_windows_arm64.zip` |

All releases include `checksums.txt` (SHA256).

### Linux / macOS

```bash
# Download (example: Linux x86_64)
curl -sSL https://github.com/lleontor705/opencode-model-selector/releases/latest/download/opencode-model-selector_linux_amd64.tar.gz | tar xz
chmod +x opencode-model-selector
sudo mv opencode-model-selector /usr/local/bin/
```

### Windows (PowerShell)

```powershell
Invoke-WebRequest -Uri https://github.com/lleontor705/opencode-model-selector/releases/latest/download/opencode-model-selector_windows_amd64.zip -OutFile opencode-model-selector.zip
Expand-Archive opencode-model-selector.zip -DestinationPath .
Move-Item opencode-model-selector.exe C:\Users\$env:USERNAME\bin\
```

## Verify Installation

```bash
opencode-model-selector --list-models
opencode-model-selector --list-agents
```

## Prerequisites

- [OpenCode CLI](https://opencode.ai) installed and on your `$PATH` (required for TUI and `--list-models`; `--list-agents` works without it)
- An `opencode.json` config file (default: `~/.config/opencode/opencode.json`)

## Windows Notes

- `go install` is recommended to avoid antivirus false positives on unsigned binaries
- If using prebuilt binaries, you may need to add an antivirus exclusion
- opencode-model-selector is pure Go (CGO disabled) — no C compiler needed
