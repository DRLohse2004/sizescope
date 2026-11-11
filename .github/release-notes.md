**Download:** `SizeScope.exe` herunterladen und starten – keine Installation, keine Abhängigkeiten.

### Neu in 2.0.3
- Als Administrator werden jetzt auch geschützte Ordner wie „System Volume Information“ gelesen (Windows-Sicherungsrecht, nur Auflisten). In den Einstellungen abschaltbar.
- Statusleiste erklärt bei Admin-Rechten, dass verbleibende Ordner auch für Administratoren gesperrt sind.
- Leere Ordner zeigen in der Karte einen Hinweis statt einer leeren Fläche.

### 2.0.2
- Hinweisleiste, wenn SizeScope ohne Administratorrechte läuft – mit „Als Administrator neu starten“. Lässt sich wegklicken oder in den Einstellungen abschalten.
- Behoben: abgeschnittene Texte in den Einstellungen.

### 2.0.1
- Behoben: Absturz beim CSV-Export.
- Behoben: In den Einstellungen ließ sich nichts anklicken.
- Netzlaufwerke: korrekte Belegung bei Freigaben mit Kontingent (keine negativen Werte mehr), eigenes Symbol, Hinweis „nicht erreichbar“ bei getrennten Laufwerken.
- Getrennte Netzlaufwerke verzögern den Programmstart nicht mehr.
- Fehler in Hintergrundaufgaben werden als Meldung angezeigt statt das Programm zu beenden.

### 2.0.0
- Native Oberfläche ohne WebView2 – läuft auch auf Windows-Servern (ab Server 2016) und per RDP.

Die EXE ist nicht signiert – Windows SmartScreen fragt beim ersten Start nach („Weitere Informationen“ → „Trotzdem ausführen“).
