# Eigene Bereinigungen

Über „Bereinigen“ → „Importieren“ lassen sich `.json`- und `.ps1`-Dateien hinzufügen. Sie landen im Ordner `cleanups` neben der EXE und können dort auch direkt abgelegt werden. Beispiele: [`examples/cleanups`](../examples/cleanups).

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

