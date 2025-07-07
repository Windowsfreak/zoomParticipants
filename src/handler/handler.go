package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
)

var (
	appState = &AppState{
		Meetings: make(map[string]*MeetingData),
	}
	tmpl *template.Template
)

// init parses the HTML template for the participant list page
func init() {
	appState.Meetings["example-meeting-uuid"] = &MeetingData{
		Participants: map[string]string{
			"11223":  "Alice Johnson",
			"44556":  "Bob Brown",
			"77889":  "Charlie Black",
			"99000":  "Diana White",
			"33445":  "Eve Green",
			"55667":  "Frank Blue",
			"88990":  "Grace Yellow",
			"12321":  "Hank Purple",
			"45654":  "Ivy Orange",
			"78987":  "Jack Pink",
			"10101":  "Kathy Cyan",
			"20202":  "Leo Magenta",
			"30303":  "Mia Teal",
			"40404":  "Nina Brown",
			"50505":  "Oscar Gray",
			"60606":  "Paul Silver",
			"70707":  "Quinn Gold",
			"80808":  "Rita Bronze",
			"90909":  "Sam White",
			"101010": "Tina Black",
			"111111": "Uma Red",
			"121212": "Vera Blue",
			"131313": "Will Green",
			"141414": "Xena Yellow",
			"151515": "Yara Pink",
			"161616": "Zane Purple",
		},
		Topic:       "Example Meeting",
		LastUpdated: time.Now(),
	}
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Meetingteilnehmer</title>
    <style>
		html, body {
            height: 100vh; /* Ensure body takes full viewport height */
			margin: 0;
			padding: 0;
        }
        body {
            font-family: Arial, sans-serif;
            transition: background-color 0.3s, color 0.3s;
        }
        @media (prefers-color-scheme: dark) {
            body {
                background-color: #121212;
                color: #e0e0e0;
            }
            input, button {
                background-color: #333;
                color: #e0e0e0;
                border: 1px solid #555;
            }
        }
        @media (prefers-color-scheme: light) {
            body {
                background-color: #ffffff;
                color: #333;
            }
            input, button {
                background-color: #f0f0f0;
                color: #333;
                border: 1px solid #ccc;
            }
        }
        h1, h2, p {
            margin: 0 0 10px 0;
        }
        .container {
            margin: 0 auto;
            height: 100vh; /* Full viewport height */
            display: flex;
            flex-direction: column;
        }
        .header {
            flex: 0 0 auto; /* Header takes only the space it needs */
            text-align: center;
			margin: 20px 0;
        }
        .password-form {
            text-align: center;
            margin-bottom: 20px;
        }
        .participants-container {
            flex: 1; /* Takes up remaining height */
            overflow-y: auto; /* Allows scrolling if list is long */
            display: flex;
            flex-direction: column;
            flex-wrap: wrap;
            gap: 2px;
			align-content: center;
			justify-content: flex-start;
        }
        .participant {
            height: 30px;
            line-height: 30px;
            flex: 0 0 auto;
            box-sizing: border-box;
            padding: 0 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            background-color: rgba(0,0,0,0.05);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        @media (prefers-color-scheme: dark) {
            .participant {
                border: 1px solid #444;
                background-color: rgba(255,255,255,0.1);
            }
        }
        @media (max-width: 600px) {
            .participant {
                width: calc(50% - 5px);
            }
        }
        .button-group {
            display: flex;
            justify-content: center;
            gap: 10px;
            margin-bottom: 10px;
        }
        button {
            padding: 10px 20px;
            cursor: pointer;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Meetingteilnehmer</h1>
            {{ if .Authenticated }}
                <h2>Meeting: {{ .MeetingTopic }}</h2>
                <p>Letzte Aktualisierung: {{ .Updated }}</p>
                <div class="button-group">
                    <button id="copy" onclick="copyToClipboard()">Liste in Zwischenablage kopieren</button>
                    <button onclick="document.getElementById('refreshForm').submit()">Aktualisieren</button>
                </div>
				<p>Teilnehmer: {{ .ParticipantCount }}</p>
                <form id="refreshForm" method="POST" action="/" style="display:none;">
                    <input type="hidden" name="password" value="{{ .Password }}" />
                </form>
            {{ end }}
        </div>
        {{ if .Authenticated }}
            <div class="participants-container">
                {{ range $index, $name := .Participants }}
                    <div class="participant">{{ $name }}</div>
                {{ end }}
            </div>
            <script>
                function copyToClipboard() {
                    const participants = document.querySelectorAll('.participant');
                    let text = Array.from(participants)
                        .map(p => p.textContent)
                        .join('\n');
                    navigator.clipboard.writeText(text)
// change the button name for a few seconds
						.then(() => {
							const button = document.querySelector('#copy');
							if (!button) return;
							button.textContent = 'In Zwischenablage kopiert!';
							setTimeout(() => {
								button.textContent = 'Liste in Zwischenablage kopieren';
							}, 2000);
						})
                        .catch(err => alert('Kopieren fehlgeschlagen: ' + err));
                }
            </script>
        {{ else }}
            <div class="password-form">
                <form method="POST" action="/">
                    <label for="password">Diese Seite ist durch ein Kennwort geschützt:</label>
                    <input type="password" id="password" name="password" required>
                    <button type="submit">Absenden</button>
                </form>
            </div>
        {{ end }}
    </div>
</body>
</html>`
	var err error
	tmpl, err = template.New("participants").Parse(html)
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}
}

func NewServer() *http.Server {
	r := httprouter.New()
	r.POST("/webhook", WebhookHandler)
	r.GET("/", ViewParticipantsHandler)
	r.POST("/", ViewParticipantsHandler)

	// Start cleanup routine
	go cleanupOldMeetings()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	addr := "localhost:" + port
	return &http.Server{
		Addr:    addr,
		Handler: r,
	}
}

// validateWebhookSignature verifies the incoming webhook signature
func validateWebhookSignature(r *http.Request, secretToken string) bool {
	timestamp := r.Header.Get("x-zm-request-timestamp")
	signature := r.Header.Get("x-zm-signature")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		return false
	}
	// Reset body for further processing
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	message := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	h := hmac.New(sha256.New, []byte(secretToken))
	h.Write([]byte(message))
	computedSignature := "v0=" + hex.EncodeToString(h.Sum(nil))

	return signature == computedSignature
}

// handleWebhookValidation handles Zoom's endpoint URL validation challenge
func handleWebhookValidation(w http.ResponseWriter, r *http.Request, payload ZoomWebhookPayload) {
	plainToken := payload.Payload.PlainToken
	h := hmac.New(sha256.New, []byte(ConfigInstance.WebhookSecretToken))
	h.Write([]byte(plainToken))
	encryptedToken := hex.EncodeToString(h.Sum(nil))

	response := map[string]string{
		"plainToken":     plainToken,
		"encryptedToken": encryptedToken,
	}
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		log.Printf("Error encoding webhook validation response: %v", err)
	}
}

// handleParticipantJoined adds a participant to the meeting data
func handleParticipantJoined(payload ZoomWebhookPayload) {
	meetingUUID := payload.Payload.Object.UUID
	participant := payload.Payload.Object.Participant
	uniqueID := participant.UserID
	if uniqueID == "" {
		uniqueID = participant.UserName
	}
	displayName := participant.UserName
	if displayName == "" {
		displayName = "Anonymous"
	}

	appState.Mutex.Lock()
	defer appState.Mutex.Unlock()

	if _, exists := appState.Meetings[meetingUUID]; !exists {
		appState.Meetings[meetingUUID] = &MeetingData{
			Participants: make(map[string]string),
			Topic:        payload.Payload.Object.Topic,
			LastUpdated:  time.Now(),
		}
	}

	meeting := appState.Meetings[meetingUUID]
	meeting.Participants[uniqueID] = displayName
	meeting.LastUpdated = time.Now()
	log.Printf("Participant joined: %s in meeting %s", displayName, meetingUUID)
}

// handleParticipantLeft removes a participant from the meeting data
func handleParticipantLeft(payload ZoomWebhookPayload) {
	meetingUUID := payload.Payload.Object.UUID
	participant := payload.Payload.Object.Participant
	uniqueID := participant.UserID
	if uniqueID == "" {
		uniqueID = participant.UserName
	}

	appState.Mutex.Lock()
	defer appState.Mutex.Unlock()

	if meeting, exists := appState.Meetings[meetingUUID]; exists {
		delete(meeting.Participants, uniqueID)
		meeting.LastUpdated = time.Now()
		log.Printf("Participant left: %s from meeting %s", participant.UserName, meetingUUID)
	}
}

// handleMeetingEnded clears all participants when the meeting ends
func handleMeetingEnded(payload ZoomWebhookPayload) {
	meetingUUID := payload.Payload.Object.UUID

	appState.Mutex.Lock()
	defer appState.Mutex.Unlock()

	if meeting, exists := appState.Meetings[meetingUUID]; exists {
		meeting.Participants = make(map[string]string)
		meeting.LastUpdated = time.Now()
		log.Printf("Meeting ended: %s", meetingUUID)
	}
}

// cleanupOldMeetings removes meeting data older than 6 hours
func cleanupOldMeetings() {
	for {
		time.Sleep(time.Hour)
		appState.Mutex.Lock()
		for uuid, meeting := range appState.Meetings {
			if time.Since(meeting.LastUpdated) > 6*time.Hour {
				delete(appState.Meetings, uuid)
				log.Printf("Cleaned up old meeting: %s", uuid)
			}
		}
		appState.Mutex.Unlock()
	}
}

// WebhookHandler processes incoming Zoom webhook events
func WebhookHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !validateWebhookSignature(r, ConfigInstance.WebhookSecretToken) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Println("Webhook signature validation failed")
		return
	}

	var payload ZoomWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		log.Printf("Error decoding webhook payload: %v", err)
		return
	}

	switch payload.Event {
	case "endpoint.url_validation":
		handleWebhookValidation(w, r, payload)
	case "meeting.participant_joined":
		handleParticipantJoined(payload)
	case "meeting.participant_left":
		handleParticipantLeft(payload)
	case "meeting.ended":
		handleMeetingEnded(payload)
	default:
		log.Printf("Unhandled event: %s", payload.Event)
	}

	w.WriteHeader(http.StatusOK)
}

// ViewParticipantsHandler displays the participant list or password prompt
func ViewParticipantsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	authenticated := false

	if r.Method == "POST" {
		password := r.FormValue("password")
		if password == ConfigInstance.ViewerPassword {
			authenticated = true
		} else {
			http.Error(w, "Incorrect password", http.StatusUnauthorized)
			return
		}
	}

	// Get the latest meeting (for simplicity, show the most recently updated)
	var latestMeeting *MeetingData
	var latestUUID string
	appState.Mutex.RLock()
	for uuid, meeting := range appState.Meetings {
		if latestMeeting == nil || meeting.LastUpdated.After(latestMeeting.LastUpdated) {
			latestMeeting = meeting
			latestUUID = uuid
		}
	}
	appState.Mutex.RUnlock()

	data := struct {
		Authenticated    bool
		Participants     []string
		ParticipantCount int
		MeetingTopic     string
		Password         string
		Updated          string
	}{
		Authenticated:    authenticated,
		Participants:     nil,
		ParticipantCount: 0,
		MeetingTopic:     "",
		Password:         "",
		Updated:          time.Now().Format(time.RFC1123),
	}

	if authenticated && latestMeeting != nil {
		names := make([]string, 0, len(latestMeeting.Participants))
		for _, name := range latestMeeting.Participants {
			names = append(names, name)
		}
		sort.Strings(names)

		data.Participants = names
		data.ParticipantCount = len(names)
		data.MeetingTopic = latestMeeting.Topic
		data.Password = r.FormValue("password")
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
	}
}
