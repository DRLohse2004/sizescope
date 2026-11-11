# SizeScope

Portabler Speicherplatz-Analysator für Windows – ähnlich wie TreeSize Free. Eine einzelne EXE ohne Installation und ohne Abhängigkeiten (kein WebView2, kein .NET), läuft daher auch auf Windows-Servern.

![Ordnerbaum und Treemap](docs/screenshot.png)

## Funktionen

- **Ordnerbaum** mit Größe, belegtem Speicher, Anteil am übergeordneten Ordner, Anzahl Dateien/Ordner und Änderungsdatum – sortierbar, füllt sich schon während des Scans
- **Treemap** des gewählten Ordners, eingefärbt nach Dateityp oder Ordner
- **Dateitypen**, **größte Dateien** (filterbar nach Endung) und **Alter** nach Änderungsdatum
- Kontextmenü: Öffnen, im Explorer zeigen, Pfad kopieren, neu einlesen, in den Papierkorb verschieben
- CSV-Export, Neustart als Administrator, Ausschlussmuster (z. B. `node_modules`)
- Deutsch/Englisch, hell/dunkel

Junctions und symbolische Links werden nicht verfolgt (keine Doppelzählung). Dateien, die nur online in OneDrive & Co. liegen, werden standardmäßig ausgelassen; Cloud-Ordner werden ohne Download gelesen. Beides ist in den Einstellungen umstellbar.

## Verwendung

`SizeScope.exe` starten, Laufwerk wählen. Optional mit Pfad starten: `SizeScope.exe D:\Daten`

Tastatur im Baum: Pfeiltasten, `Enter` öffnen, `F5` neu einlesen, `Entf` Papierkorb, `Strg+C` Pfad kopieren.

**Voraussetzung:** Windows 10/11 oder Windows Server 2016 und neuer (64 Bit). Gezeichnet wird mit Direct3D 11; ohne Grafiktreiber übernimmt Windows das per Software.

**Portabel:** Einstellungen liegen als `SizeScope.settings.json` neben der EXE (falls der Ordner schreibgeschützt ist, unter `%APPDATA%\SizeScope`).

## Bauen

Benötigt nur Go, funktioniert auch von Linux aus:

```
go install github.com/tc-hib/go-winres@latest
go-winres make --arch amd64
GOOS=windows go build -trimpath -ldflags "-H windowsgui -s -w" -o build/bin/SizeScope.exe .
```

**Release:** Versionsnummer in `VERSION` erhöhen und auf `main` pushen – GitHub Actions baut die EXE und legt das Release `v<Version>` an. Text dafür: `.github/release-notes.md`.

## Aufbau

| Pfad | Inhalt |
|---|---|
| `scanner/` | Paralleler Scan, Baum-Modell, Abfragen (Treemap, Typen, Top-Dateien, Export) |
| `ui_*.go` | Oberfläche mit [Gio](https://gioui.org) |
| `platform_*.go`, `ui_windows.go` | Laufwerke, Papierkorb, Explorer, Dialoge, Admin-Neustart |
| `winres/` | Icon, Manifest und Versionsinfo der EXE |
