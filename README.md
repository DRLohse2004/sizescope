<p align="center">
  <img src="docs/icon.png" width="72" alt="">
</p>

<h1 align="center">SizeScope</h1>

<p align="center">
  Portabler Speicherplatz-Analysator für Windows – eine EXE, keine Installation, keine Abhängigkeiten.
</p>

<p align="center">
  <a href="https://github.com/DRLohse2004/sizescope/releases/latest"><img src="https://img.shields.io/github/v/release/DRLohse2004/sizescope?label=Download" alt="Download"></a>
  <img src="https://img.shields.io/badge/Windows-10%20%7C%2011%20%7C%20Server%202016%2B-0078D4" alt="Windows">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/DRLohse2004/sizescope" alt="Lizenz"></a>
</p>

![SizeScope mit Ordnerbaum und Treemap](docs/screenshot.png)

## Funktionen

- **Ordnerbaum** mit Größe, belegtem Speicher, Anteil, Dateien und Änderungsdatum – füllt sich schon während des Scans
- **Treemap**, **Dateitypen**, **größte Dateien** und **Alter** für den gewählten Ordner
- **Bereinigen**: Temp-Ordner, Update-Downloads, Caches, Papierkorb u. v. m. – mit Vorschau und Aufstellung, was wo gelöscht wurde; [eigene Regeln und Skripte](docs/cleanups.md) importierbar
- Liest als Administrator auch geschützte Ordner wie *System Volume Information*
- Erkennt OneDrive-Dateien, die nur online liegen, und lässt sie auf Wunsch aus
- Kontextmenü, Papierkorb, CSV-Export, Tastatursteuerung
- Deutsch/Englisch, hell/dunkel; Einstellungen liegen neben der EXE

## Start

[`SizeScope.exe` herunterladen](https://github.com/DRLohse2004/sizescope/releases/latest), starten, Laufwerk wählen. Optional mit Pfad: `SizeScope.exe D:\Daten`

Läuft auf Windows 10/11 und Windows Server ab 2016, auch per RDP. Die EXE ist nicht signiert; SmartScreen fragt daher beim ersten Start nach.

## Selbst bauen

Benötigt nur [Go](https://go.dev) – funktioniert auch unter Linux:

```sh
go install github.com/tc-hib/go-winres@latest
go-winres make --arch amd64
GOOS=windows go build -trimpath -ldflags "-H windowsgui -s -w" -o SizeScope.exe .
```

| Pfad | Inhalt |
|---|---|
| `scanner/` | Paralleler Scan und Abfragen (Treemap, Typen, Export) |
| `cleanup/` | Bereinigungs-Engine und mitgelieferte Regeln |
| `ui_*.go` | Oberfläche mit [Gio](https://gioui.org) |
| `platform_*.go`, `ui_windows.go` | Windows-Funktionen (Laufwerke, Papierkorb, Dialoge) |

Ein neues Release entsteht automatisch, wenn die Versionsnummer in `VERSION` erhöht wird.

## Mitmachen

Fehler und Ideen gerne als [Issue](https://github.com/DRLohse2004/sizescope/issues). Es gilt der [Verhaltenskodex](CODE_OF_CONDUCT.md).

## Lizenz

[MIT](LICENSE). Verwendet [Gio](https://gioui.org) (MIT/Unlicense), Go-Bibliotheken unter `golang.org/x` und die Go-Schriftarten (BSD-3-Clause) sowie [Material Design Icons](https://github.com/google/material-design-icons) (Apache 2.0).
