# SizeScope

Portabler Speicherplatz-Analysator für Windows – ähnlich wie TreeSize Free. Eine einzelne EXE ohne Installation und ohne Abhängigkeiten (kein WebView2, kein .NET), läuft daher auch auf Windows-Servern.

![Ordnerbaum und Treemap](docs/screenshot.png)

## Funktionen

- **Ordnerbaum** mit Größe, belegtem Speicher, Anteil am übergeordneten Ordner, Anzahl Dateien/Ordner und Änderungsdatum – sortierbar, füllt sich schon während des Scans
- **Treemap** des gewählten Ordners, eingefärbt nach Dateityp oder Ordner
- **Dateitypen**, **größte Dateien** (filterbar nach Endung) und **Alter** nach Änderungsdatum
- Kontextmenü: Öffnen, im Explorer zeigen, Pfad kopieren, neu einlesen, in den Papierkorb verschieben
- **Bereinigen**: Temp-Ordner, Update-Downloads, Caches, Papierkorb u. v. m. – mit Vorschau und Aufstellung, was wo gelöscht wurde; eigene Bereinigungen importierbar
- CSV-Export, Neustart als Administrator, Ausschlussmuster (z. B. `node_modules`)
- Deutsch/Englisch, hell/dunkel

Junctions und symbolische Links werden nicht verfolgt (keine Doppelzählung). Dateien, die nur online in OneDrive & Co. liegen, werden standardmäßig ausgelassen; Cloud-Ordner werden ohne Download gelesen. Beides ist in den Einstellungen umstellbar.

## Verwendung

`SizeScope.exe` starten, Laufwerk wählen. Optional mit Pfad starten: `SizeScope.exe D:\Daten`

Tastatur im Baum: Pfeiltasten, `Enter` öffnen, `F5` neu einlesen, `Entf` Papierkorb, `Strg+C` Pfad kopieren.

**Voraussetzung:** Windows 10/11 oder Windows Server 2016 und neuer (64 Bit). Gezeichnet wird mit Direct3D 11; ohne Grafiktreiber übernimmt Windows das per Software.

**Portabel:** Einstellungen liegen als `SizeScope.settings.json` neben der EXE (falls der Ordner schreibgeschützt ist, unter `%APPDATA%\SizeScope`).

## Eigene Bereinigungen

Über „Bereinigen“ → „Importieren“ lassen sich `.json`- und `.ps1`-Dateien hinzufügen. Sie landen im Ordner `cleanups` neben der EXE und können dort auch direkt abgelegt werden. Beispiele: [`examples/cleanups`](examples/cleanups).

**JSON-Regel** – SizeScope löscht selbst, daher mit Vorschau und genauer Aufstellung:

```json
{
  "name": { "de": "Mein Cache", "en": "My cache" },
  "description": "Optional",
  "admin": false,
  "stopServices": ["dienstname"],
  "targets": [
    { "path": "%LOCALAPPDATA%\\App\\*\\Cache", "include": ["*"], "exclude": ["*.keep"],
      "minAgeDays": 7, "recurse": true, "removeEmptyDirs": true }
  ],
  "commands": ["optionaler Befehl, Platzgewinn wird über den freien Speicher gemessen"]
}
```

`path` darf `%VARIABLEN%` und `*` in jedem Abschnitt enthalten. Laufwerkswurzeln und Ordner direkt darunter (z. B. `C:\Windows`) werden abgelehnt. Verknüpfungen und Junctions werden nie verfolgt.

**PowerShell-Skript** – optionale Kopfzeilen: `# Name:`, `# Name-de:`, `# Description:`, `# Admin: true`, `# Slow: true`, `# Preview: true`. Gibt das Skript Zeilen `SIZESCOPE|<Ort>|<Bytes>|<Dateien>` aus, erscheinen sie in der Übersicht; sonst zählt der Zuwachs an freiem Speicher. In der Vorschau läuft ein Skript nur mit `# Preview: true`, dann ist `SIZESCOPE_DRYRUN=1` gesetzt.

Gelöscht wird endgültig, nicht in den Papierkorb.

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
