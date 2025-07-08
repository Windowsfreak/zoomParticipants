# ZoomParticipants

Ein Open-Source-Tool zur Echtzeit-Auswertung der Teilnehmerliste in Zoom-Meetings. Diese Anwendung ermöglicht es Ihnen, Teilnehmerlisten während eines Meetings zu extrahieren.

## Funktionen

- **Echtzeit-Teilnehmererfassung**: Erfasst Teilnehmerdaten während eines Zoom-Meetings über Webhooks.
- **Datenschutzorientiert**: Teilnehmernamen werden nur temporär im Speicher gehalten und spätestens nach 6 Stunden, dem Verlassen oder Meeting-Ende gelöscht.
- **Multi-User-Unterstützung**: Unterstützt mehrere Zoom-Konten mit individuellen Secret Tokens und Viewer-Passwörtern.
- **Benutzerfreundliche Oberfläche**: Eine einfache Weboberfläche zum Anzeigen und Kopieren der Teilnehmerliste.

## Voraussetzungen

- **Golang**: Zum Kompilieren der Anwendung.
- **CGO**: Erforderlich für die SQLite-Integration.
- **SQLite-Treiber**: Für die Speicherung von Kontoinformationen.
- **Reverse-Proxy-Webserver**: Für HTTPS-Unterstützung.

## Installation

1. Repository klonen:

```bash
git clone https://github.com/Windowsfreak/zoomParticipants.git
cd zoomParticipants
```

2. Binary erstellen:

```bash
make build
```

3. Server starten (Alternativ kann ein Daemon eingerichtet werden):

```bash
./bin/main
```

4. Reverse-Proxy für HTTPS-Unterstützung einrichten.

## Einrichtung eines neuen Benutzers

- **Account-ID finden**: Melden Sie sich auf der Zoom-Website an, öffnen Sie die Entwickler-Tools im Browser und suchen Sie nach dem HTTP-only-Cookie `zm_aid`..
- **Secret Token**: Wird beim Hinzufügen Ihrer Anwendung im Zoom App Marketplace bereitgestellt.
- **Viewer-Passwort**: Wählen Sie ein Passwort, das beim Hinzufügen des Kontos verwendet wird.
- Fügen Sie das Konto über die Weboberfläche hinzu, indem Sie die Account-ID, den Secret Token und das Viewer-Passwort eingeben.

## Datenschutz und Sicherheit

- Die o.a. Daten werden in einer SQLite-Datenbank gespeichert. So wird der Empfang von Webhooks sichergestellt.
- Persönliche Informationen wie Teilnehmernamen werden nur vorübergehend gespeichert.
- Es werden keine Aufzeichnungen oder Logs über Teilnehmerdaten erstellt.
- Der Secret Token wird zu Authentifizierungszwecken in der Datenbank gespeichert. Daher muss diese vor externem Zugriff geschützt sein.
- Mit dem Secret Token ist kein Zugriff auf Zoom möglich, da es sich nur um eine Webhook-Authentifizierung handelt.

## Kontakt

Für Unterstützung oder Fragen erreichen Sie mich über Facebook, Instagram und X. Die Links finden Sie unter [https://8bj.de](https://8bj.de).

## Lizenz

Dieses Projekt ist Open Source und steht unter der MIT-Lizenz.
