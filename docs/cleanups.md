# Custom cleanups

*[Deutsch](cleanups.de.md)*

Use "Clean up" → "Import" to add `.json` and `.ps1` files. They are stored in the `cleanups` folder next to the EXE, where you can also place them directly. Examples: [`examples/cleanups`](../examples/cleanups).

**JSON rule** – SizeScope deletes the files itself, so preview and a detailed report are available:

```json
{
  "name": { "de": "Mein Cache", "en": "My cache" },
  "description": "Optional",
  "admin": false,
  "stopServices": ["servicename"],
  "targets": [
    { "path": "%LOCALAPPDATA%\\App\\*\\Cache", "include": ["*"], "exclude": ["*.keep"],
      "minAgeDays": 7, "recurse": true, "removeEmptyDirs": true }
  ],
  "commands": ["optional command; freed space is measured via free disk space"]
}
```

`path` may contain `%VARIABLES%` and `*` in any segment. Drive roots and folders directly below them (e.g. `C:\Windows`) are refused. Links and junctions are never followed. If a variable is not set, the target is skipped.

**PowerShell script** – optional header lines: `# Name:`, `# Name-en:`, `# Name-de:`, `# Description:`, `# Admin: true`, `# Slow: true`, `# Preview: true`. Lines printed as `SIZESCOPE|<location>|<bytes>|<files>` appear in the report; otherwise the gain in free disk space is shown. In preview mode a script only runs if it declares `# Preview: true`; `SIZESCOPE_DRYRUN=1` is set then.

Files are deleted permanently, not moved to the recycle bin.
