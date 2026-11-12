<p align="center">
  <img src="docs/icon.png" width="72" alt="">
</p>

<h1 align="center">SizeScope</h1>

<p align="center">
  Portable disk space analyzer for Windows – one EXE, no installation, no dependencies.
</p>

<p align="center">
  <a href="https://github.com/DRLohse2004/sizescope/releases/latest"><img src="https://img.shields.io/github/v/release/DRLohse2004/sizescope?label=Download" alt="Download"></a>
  <img src="https://img.shields.io/badge/Windows-10%20%7C%2011%20%7C%20Server%202016%2B-0078D4" alt="Windows">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/DRLohse2004/sizescope" alt="License"></a>
</p>

<p align="center"><b>English</b> · <a href="README.de.md">Deutsch</a></p>

![SizeScope with folder tree and treemap](docs/screenshot.png)

## Features

- **Folder tree** with size, allocated space, share, file count and modification date – fills in while the scan is still running
- **Treemap**, **file types**, **largest files** and **age** for the selected folder
- **Clean up**: temp folders, update downloads, caches, recycle bin and more – with preview and a report of what was deleted where; [custom rules and scripts](docs/cleanups.md) can be imported
- Reads protected folders such as *System Volume Information* when run as administrator
- Detects OneDrive files that are only stored online and can leave them out
- Context menu, recycle bin, CSV export, keyboard navigation
- English/German, light/dark; settings are stored next to the EXE

## Getting started

[Download `SizeScope.exe`](https://github.com/DRLohse2004/sizescope/releases/latest), run it, pick a drive. Optionally pass a path: `SizeScope.exe D:\Data`

Runs on Windows 10/11 and Windows Server 2016 or later, including over RDP. The EXE is not signed, so SmartScreen asks for confirmation on first start.

## Building

Only [Go](https://go.dev) is required – works on Linux too:

```sh
go install github.com/tc-hib/go-winres@latest
go-winres make --arch amd64
GOOS=windows go build -trimpath -ldflags "-H windowsgui -s -w" -o SizeScope.exe .
```

| Path | Contents |
|---|---|
| `scanner/` | Parallel scan and queries (treemap, types, export) |
| `cleanup/` | Cleanup engine and built-in rules |
| `ui_*.go` | User interface with [Gio](https://gioui.org) |
| `platform_*.go`, `ui_windows.go` | Windows functions (drives, recycle bin, dialogs) |

A new release is built automatically when the version in `VERSION` is increased.

## Contributing

Bug reports and ideas are welcome as [issues](https://github.com/DRLohse2004/sizescope/issues). Please follow the [code of conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE). Uses [Gio](https://gioui.org) (MIT/Unlicense), Go libraries under `golang.org/x` and the Go fonts (BSD-3-Clause), and [Material Design Icons](https://github.com/google/material-design-icons) (Apache 2.0).
