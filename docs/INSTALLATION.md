# Installation

## go install (Recommended)

```bash
go install github.com/lleontor705/opencode-model-selector/cmd/ocs@latest
```

Binary goes to `$GOPATH/bin/ocs` (typically `~/go/bin/` or `%USERPROFILE%\go\bin\`). The repo/module is still `github.com/lleontor705/opencode-model-selector`; the entrypoint lives at `cmd/ocs`, so only the installed binary is named `ocs`.

> **Upgrading from a pre-`cmd/ocs` install:** earlier versions installed a binary named `opencode-model-selector` (or `opencode-model-selector.exe` on Windows) into `$GOPATH/bin`, because `go install .` named it after the module. That binary is now **stale** — the canonical name is `ocs`. You can safely delete the old `$GOPATH/bin/opencode-model-selector(.exe)`; it will not be updated by the new install command.

## Build from Source

```bash
git clone https://github.com/lleontor705/opencode-model-selector.git
cd opencode-model-selector
go build -ldflags="-s -w" -o ocs ./cmd/ocs
```

With version stamp:

```bash
go build -ldflags="-s -w -X main.version=local-$(git describe --tags --always)" -o ocs ./cmd/ocs
```

## Pre-built Binaries

Download from [Releases](https://github.com/lleontor705/opencode-model-selector/releases):

| Platform | File |
|----------|------|
| Linux x86_64 | `ocs_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `ocs_<version>_linux_arm64.tar.gz` |
| macOS Intel | `ocs_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `ocs_<version>_darwin_arm64.tar.gz` |
| Windows x86_64 | `ocs_<version>_windows_amd64.zip` |
| Windows ARM64 | `ocs_<version>_windows_arm64.zip` |

All releases include `checksums.txt` (SHA256). Asset names follow goreleaser's `ocs_` template and extract a single binary named `ocs` (`ocs.exe` on Windows).

### Linux / macOS

```bash
# Download (example: Linux x86_64)
curl -sSL https://github.com/lleontor705/opencode-model-selector/releases/latest/download/ocs_linux_amd64.tar.gz | tar xz
chmod +x ocs
sudo mv ocs /usr/local/bin/
```

### Windows (PowerShell)

```powershell
Invoke-WebRequest -Uri https://github.com/lleontor705/opencode-model-selector/releases/latest/download/ocs_windows_amd64.zip -OutFile ocs.zip
Expand-Archive ocs.zip -DestinationPath .
Move-Item ocs.exe C:\Users\$env:USERNAME\bin\
```

## Verify Installation

```bash
ocs --list-models
# or
ocs --help
```

## Prerequisites

- [OpenCode CLI](https://opencode.ai) installed and on your `$PATH` (required for TUI and `--list-models`; `--list-agents` works without it)
- An OpenCode config file. Both `opencode.json` and `opencode.jsonc` are supported. The default locations (probed in this order) are `~/.config/opencode/opencode.jsonc` and `~/.config/opencode/opencode.json`. If neither exists, the tool defaults to creating `opencode.json` on first save.

  > **Note on JSONC:** `.jsonc` comments and trailing commas are supported on **load** but are **NOT preserved when the tool saves the config** — it writes standard JSON. If you rely on comments, keep them in a version-controlled copy; otherwise they will be lost the first time `ocs` writes the file.

## Windows Notes

- `go install` is recommended to avoid antivirus false positives on unsigned binaries
- If using prebuilt binaries, you may need to add an antivirus exclusion
- The binary is `ocs.exe` (invoked as `ocs` once on your `PATH`); `ocs` is pure Go (CGO disabled) — no C compiler needed
