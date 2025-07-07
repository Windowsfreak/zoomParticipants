package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	_ "github.com/mattn/go-sqlite3"
)

var (
	appState = &AppState{
		Meetings:            make(map[string]map[string]*MeetingData),
		PasswordToAccountID: make(map[string]string),
	}
	tmpl *template.Template
)

// init parses the HTML template for the participant list page
func init() {
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
		.add-account-form {
			text-align: center;
			margin-top: 20px;
		}
		.add-account-form div {
			margin-bottom: 10px;
		}
		.add-account-form input {
			padding: 5px;
			width: 250px;
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
			<div class="add-account-form">
				<h3>Add New Account</h3>
				<form method="POST" action="/add-account">
					<div>
						<label for="account_id">Account ID:</label>
						<input type="text" id="account_id" name="account_id" required>
					</div>
					<div>
						<label for="secret_token">Secret Token (min 15 chars):</label>
						<input type="password" id="secret_token" name="secret_token" required minlength="15">
					</div>
					<div>
						<label for="viewer_password">Viewer Password (min 15 chars, unique):</label>
						<input type="password" id="viewer_password" name="viewer_password" required minlength="15">
					</div>
					<button type="submit">Add Account</button>
					{{ if .ErrorMessage }}
						<p style="color: red;">{{ .ErrorMessage }}</p>
					{{ end }}
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

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	createTableSQL := `
    CREATE TABLE IF NOT EXISTS accounts (
        account_id TEXT PRIMARY KEY,
        secret_token TEXT NOT NULL,
        viewer_password TEXT NOT NULL UNIQUE
    );`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %v", err)
	}

	appState.DB = db

	return db, nil
}

func NewServer() *http.Server {
	r := httprouter.New()
	r.POST("/webhook", WebhookHandler)
	r.GET("/", ViewParticipantsHandler)
	r.POST("/", ViewParticipantsHandler)
	r.POST("/add-account", addAccountHandler)

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
func handleWebhookValidation(w http.ResponseWriter, r *http.Request, payload ZoomWebhookPayload, secretToken string) {
	plainToken := payload.Payload.PlainToken
	h := hmac.New(sha256.New, []byte(secretToken))
	h.Write([]byte(plainToken))
	encryptedToken := hex.EncodeToString(h.Sum(nil))

	response := map[string]string{
		"plainToken":     plainToken,
		"encryptedToken": encryptedToken,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleParticipantJoined adds a participant to the meeting data
func handleParticipantJoined(payload ZoomWebhookPayload, accountID string) {
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

	if _, exists := appState.Meetings[accountID][meetingUUID]; !exists {
		appState.Meetings[accountID][meetingUUID] = &MeetingData{
			Participants: make(map[string]string),
			Topic:        payload.Payload.Object.Topic,
			LastUpdated:  time.Now(),
		}
	}

	meeting := appState.Meetings[accountID][meetingUUID]
	meeting.Participants[uniqueID] = displayName
	meeting.LastUpdated = time.Now()
}

// handleParticipantLeft removes a participant from the meeting data
func handleParticipantLeft(payload ZoomWebhookPayload, accountID string) {
	meetingUUID := payload.Payload.Object.UUID
	participant := payload.Payload.Object.Participant
	uniqueID := participant.UserID
	if uniqueID == "" {
		uniqueID = participant.UserName
	}

	appState.Mutex.Lock()
	defer appState.Mutex.Unlock()

	if meeting, exists := appState.Meetings[accountID][meetingUUID]; exists {
		delete(meeting.Participants, uniqueID)
		meeting.LastUpdated = time.Now()
	}
}

// handleMeetingEnded clears all participants when the meeting ends
func handleMeetingEnded(payload ZoomWebhookPayload, accountID string) {
	meetingUUID := payload.Payload.Object.UUID

	appState.Mutex.Lock()
	defer appState.Mutex.Unlock()

	if meeting, exists := appState.Meetings[accountID][meetingUUID]; exists {
		meeting.Participants = make(map[string]string)
		meeting.LastUpdated = time.Now()
	}
}

// cleanupOldMeetings removes meeting data older than 6 hours
func cleanupOldMeetings() {
	for {
		time.Sleep(time.Hour)
		appState.Mutex.Lock()
		for _, meetings := range appState.Meetings {
			for uuid, meeting := range meetings {
				if time.Since(meeting.LastUpdated) > 6*time.Hour {
					delete(appState.Meetings, uuid)
				}
			}
		}
		appState.Mutex.Unlock()
	}
}

// WebhookHandler processes incoming Zoom webhook events
func WebhookHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var payload ZoomWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	accountID := payload.Payload.AccountID
	var secretToken string
	err := appState.DB.QueryRow("SELECT secret_token FROM accounts WHERE account_id = ?", accountID).Scan(&secretToken)
	if err == sql.ErrNoRows {
		http.Error(w, "Unknown account", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if !validateWebhookSignature(r, secretToken) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Printf("Invalid signature for account %s, payload %v+", accountID, payload)
		return
	}

	appState.Mutex.Lock()
	if _, exists := appState.Meetings[accountID]; !exists {
		appState.Meetings[accountID] = make(map[string]*MeetingData)
		// Fetch viewer password to populate PasswordToAccountID map
		var viewerPassword string
		err := appState.DB.QueryRow("SELECT viewer_password FROM accounts WHERE account_id = ?", accountID).Scan(&viewerPassword)
		if err == nil {
			appState.PasswordToAccountID[viewerPassword] = accountID
		}
	}
	appState.Mutex.Unlock()

	// Process events as before, scoped to accountID
	switch payload.Event {
	case "endpoint.url_validation":
		handleWebhookValidation(w, r, payload, secretToken)
	case "meeting.participant_joined":
		handleParticipantJoined(payload, accountID)
	case "meeting.participant_left":
		handleParticipantLeft(payload, accountID)
	case "meeting.ended":
		handleMeetingEnded(payload, accountID)
	default:
	}

	w.WriteHeader(http.StatusOK)
}

// ViewParticipantsHandler displays the participant list or password prompt
func ViewParticipantsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	authenticated := false
	var accountID string
	var errorMessage string
	viewerPassword := r.FormValue("password")
	if r.Method == "POST" {
		appState.Mutex.RLock()
		if accID, exists := appState.PasswordToAccountID[viewerPassword]; exists {
			authenticated = true
			accountID = accID
		} else {
			// Fallback to database check if not in map
			var dbAccountID string
			err := appState.DB.QueryRow("SELECT account_id FROM accounts WHERE viewer_password = ?", viewerPassword).Scan(&dbAccountID)
			if err == nil {
				authenticated = true
				accountID = dbAccountID
				appState.Mutex.RUnlock()
				appState.Mutex.Lock()
				appState.PasswordToAccountID[viewerPassword] = accountID
				appState.Mutex.Unlock()
				appState.Mutex.RLock()
			} else if !errors.Is(err, sql.ErrNoRows) {
				errorMessage = "Database error during authentication."
			} else {
				errorMessage = "Incorrect password."
			}
			appState.Mutex.RUnlock()
		}
	}

	// Get the latest meeting (for simplicity, show the most recently updated)
	var latestMeeting *MeetingData
	appState.Mutex.RLock()
	for _, meeting := range appState.Meetings[accountID] {
		if latestMeeting == nil || meeting.LastUpdated.After(latestMeeting.LastUpdated) {
			latestMeeting = meeting
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
		ErrorMessage     string
	}{
		Authenticated:    authenticated,
		Participants:     nil,
		ParticipantCount: 0,
		MeetingTopic:     "",
		Password:         "",
		Updated:          time.Now().Format(time.RFC1123),
		ErrorMessage:     errorMessage,
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
		data.Password = viewerPassword
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

func addAccountHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	accountID := r.FormValue("account_id")
	secretToken := r.FormValue("secret_token")
	viewerPassword := r.FormValue("viewer_password")

	// Validate input lengths
	if len(secretToken) < 15 || len(viewerPassword) < 15 {
		renderError(w, "Secret Token and Viewer Password must be at least 15 characters long.", http.StatusBadRequest)
		return
	}

	// Insert into database with uniqueness check for viewer_password
	insertSQL := `INSERT INTO accounts (account_id, secret_token, viewer_password) VALUES (?, ?, ?)`
	_, err := appState.DB.Exec(insertSQL, accountID, secretToken, viewerPassword)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			renderError(w, "Viewer Password must be unique. This password is already in use.", http.StatusBadRequest)
		} else {
			renderError(w, fmt.Sprintf("Failed to add account: %v", err), http.StatusInternalServerError)
		}
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func renderError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	data := struct {
		Authenticated bool
		ErrorMessage  string
	}{
		Authenticated: false,
		ErrorMessage:  message,
	}
	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}
