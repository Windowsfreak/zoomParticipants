# Datenschutzrichtlinie

Ich lege großen Wert auf den Schutz Ihrer Daten. Diese Datenschutzrichtlinie erläutert, wie dieser Dienst Informationen sammelt, verwendet und schützt, die im Zusammenhang mit der Nutzung dieser Anwendung stehen.

## Welche Daten werden gesammelt?

- **Kontoinformationen**: Wenn Sie ein Konto hinzufügen, speichert die Anwendung Ihre Zoom-Account-ID, den Secret Token und das Viewer-Passwort in einer lokalen SQLite-Datenbank.
- **Teilnehmerdaten**: Namen von Meeting-Teilnehmern werden während eines Meetings ausschließlich im Speicher gehalten und nicht dauerhaft gespeichert.

## Wie verwende ich Ihre Daten?

- **Kontoinformationen**: Diese Daten werden verwendet, um Webhook-Anfragen von Zoom zu validieren und den Zugriff auf die Teilnehmerliste zu sichern.
- **Teilnehmerdaten**: Diese werden nur zur Anzeige und zum Kopieren der Teilnehmerliste während eines Meetings verwendet.

## Datenspeicherung und -löschung

- **Kontoinformationen**: Account-ID, Secret Token und Viewer-Passwort werden dauerhaft in der SQLite-Datenbank gespeichert, bis sie manuell entfernt werden.
- **Teilnehmerdaten**: Diese werden im Speicher gehalten und automatisch nach 6 Stunden Inaktivität, dem Verlassen oder Beenden des Meetings gelöscht.
- **Logs**: Es werden keine Logs generiert, um Ihre Privatsphäre zu schützen.

## Datensicherheit

- Die SQLite-Datenbank speichert sensible Informationen lokal auf Ihrem Server. Es liegt in Ihrer Verantwortung, den Zugriff auf diesen Server zu sichern.
- Selbst bei einem Diebstahl des Secret Tokens müsste ein Angreifer die Webhook-URL in Ihren Zoom-Einstellungen ändern, um Schaden anzurichten, was zusätzliche Sicherheit bietet.

## Weitergabe von Daten

Ich gebe keine Daten an Dritte weiter. Alle Informationen bleiben lokal auf Ihrem Server.

## Kontakt

Bei Fragen zum Datenschutz erreichen Sie mich über Facebook, Instagram oder X. Die Links finden Sie unter [https://8bj.de](https://8bj.de).

## Änderungen dieser Richtlinie

Diese Datenschutzrichtlinie kann aktualisiert werden. Änderungen werden im Repository veröffentlicht.
