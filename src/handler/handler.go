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
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
)

var (
	appState = &AppState{
		Meetings:            make(map[string]map[string]*MeetingData),
		AccountMutexes:      make(map[string]*sync.RWMutex),
		PasswordToAccountID: make(map[string]string),
		PasswordMutex:       sync.RWMutex{},
	}
	tmpl *template.Template
)

// Init parses the HTML template for the participant list page
func Init() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
	}
	var err error
	tmpl, err = template.New("content.gohtml").Funcs(funcMap).ParseFiles("content.gohtml")
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}
}

// InitDB initializes the SQLite database and creates the accounts table if it doesn't exist
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

	return db, nil
}

func NewServer(db *sql.DB) *http.Server {
	Init()
	r := httprouter.New()

	SetupHandlers(r, db)

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
func validateWebhookSignature(r *http.Request, body []byte, secretToken string) bool {
	timestamp := r.Header.Get("x-zm-request-timestamp")
	signature := r.Header.Get("x-zm-signature")

	message := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	h := hmac.New(sha256.New, []byte(secretToken))
	h.Write([]byte(message))
	computedSignature := "v0=" + hex.EncodeToString(h.Sum(nil))

	return signature == computedSignature
}

// handleWebhookValidation handles Zoom's endpoint URL validation challenge
func handleWebhookValidation(w http.ResponseWriter, payload ZoomWebhookPayload, secretToken string) {
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

	accountMutex := appState.AccountMutexes[accountID]
	accountMutex.Lock()
	defer accountMutex.Unlock()

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

	accountMutex := appState.AccountMutexes[accountID]
	accountMutex.Lock()
	defer accountMutex.Unlock()

	if meeting, exists := appState.Meetings[accountID][meetingUUID]; exists {
		delete(meeting.Participants, uniqueID)
		meeting.LastUpdated = time.Now()
	}
}

// handleMeetingEnded clears all participants when the meeting ends
func handleMeetingEnded(payload ZoomWebhookPayload, accountID string) {
	meetingUUID := payload.Payload.Object.UUID

	accountMutex := appState.AccountMutexes[accountID]
	accountMutex.Lock()
	defer accountMutex.Unlock()

	if meeting, exists := appState.Meetings[accountID][meetingUUID]; exists {
		meeting.Participants = make(map[string]string)
		meeting.LastUpdated = time.Now()
	}
}

// webhookHandler processes incoming Zoom webhook events
func webhookHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	var payload ZoomWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	accountID := payload.Payload.AccountID
	var secretToken string
	err = appState.DB.QueryRow("SELECT secret_token FROM accounts WHERE account_id = ?", accountID).Scan(&secretToken)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Unknown account", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if !validateWebhookSignature(r, body, secretToken) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Printf("Webhook signature validation failed for account: %s", accountID)
		return
	}

	// Initialize or get account-specific mutex (using password mutex for initialization)
	appState.PasswordMutex.Lock()
	if _, exists := appState.AccountMutexes[accountID]; !exists {
		appState.AccountMutexes[accountID] = &sync.RWMutex{}
		appState.Meetings[accountID] = make(map[string]*MeetingData)
		// Fetch viewer password to populate PasswordToAccountID map
		var viewerPassword string
		err := appState.DB.QueryRow("SELECT viewer_password FROM accounts WHERE account_id = ?", accountID).Scan(&viewerPassword)
		if err == nil {
			appState.PasswordToAccountID[viewerPassword] = accountID
		}
	}
	appState.PasswordMutex.Unlock()

	// Process event with account-specific lock
	switch payload.Event {
	case "endpoint.url_validation":
		handleWebhookValidation(w, payload, secretToken)
	case "meeting.participant_joined":
		handleParticipantJoined(payload, accountID)
	case "meeting.participant_left":
		handleParticipantLeft(payload, accountID)
	case "meeting.ended":
		handleMeetingEnded(payload, accountID)
	default:
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// addAccountHandler handles adding a new account
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
		renderError(w, "Secret Token und Viewer-Passwort müssen mindestens 15 Zeichen lang sein.")
		return
	}

	// Insert into database with uniqueness check for viewer_password
	insertSQL := `INSERT INTO accounts (account_id, secret_token, viewer_password) VALUES (?, ?, ?)`
	_, err := appState.DB.Exec(insertSQL, accountID, secretToken, viewerPassword)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			renderError(w, "Das Viewer-Passwort ist nicht sicher genug.")
		} else {
			renderError(w, fmt.Sprintf("Fehler beim Hinzufügen des Kontos: %v", err))
		}
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// viewParticipantsHandler displays the participant list or password prompt
func viewParticipantsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	authenticated := false
	var accountID string
	var errorMessage string

	if r.Method == "POST" {
		viewerPassword := r.FormValue("password")
		appState.PasswordMutex.RLock()
		if accID, exists := appState.PasswordToAccountID[viewerPassword]; exists {
			authenticated = true
			accountID = accID
		}
		appState.PasswordMutex.RUnlock()

		if !authenticated {
			// Fallback to database check
			var dbAccountID string
			err := appState.DB.QueryRow("SELECT account_id FROM accounts WHERE viewer_password = ?", viewerPassword).Scan(&dbAccountID)
			if err == nil {
				authenticated = true
				accountID = dbAccountID
				appState.PasswordMutex.Lock()
				appState.PasswordToAccountID[viewerPassword] = accountID
				appState.PasswordMutex.Unlock()
			} else if !errors.Is(err, sql.ErrNoRows) {
				errorMessage = "Datenbankfehler bei der Authentifizierung."
			} else {
				errorMessage = "Falsches Passwort."
			}
		}
	}

	if authenticated {
		accountMutex, exists := appState.AccountMutexes[accountID]
		if exists {
			accountMutex.RLock()
			defer accountMutex.RUnlock()

			// Get the latest meeting
			var latestMeeting *MeetingData
			var latestUUID string
			for uuid, meeting := range appState.Meetings[accountID] {
				if latestMeeting == nil || meeting.LastUpdated.After(latestMeeting.LastUpdated) {
					latestMeeting = meeting
					latestUUID = uuid
				}
			}

			if latestMeeting != nil {
				names := make([]string, 0, len(latestMeeting.Participants))
				for _, name := range latestMeeting.Participants {
					names = append(names, name)
				}
				sort.Strings(names)

				renderTemplate(w, true, names, len(names), latestMeeting.Topic, r.FormValue("password"), "", latestMeeting.LastUpdated.Format("2006-01-02 15:04:05"))
				log.Printf("Displaying participants for meeting: %s", latestUUID)
				return
			}
		}
	}

	renderTemplate(w, authenticated, nil, 0, "", r.FormValue("password"), errorMessage, "")
}

// renderTemplate renders the HTML template with the given data
func renderTemplate(w http.ResponseWriter, authenticated bool, participants []string, count int, topic, password, errorMsg, updated string) {
	data := struct {
		Authenticated    bool
		Participants     []string
		ParticipantCount int
		MeetingTopic     string
		Password         string
		ErrorMessage     string
		Updated          string
	}{
		Authenticated:    authenticated,
		Participants:     participants,
		ParticipantCount: count,
		MeetingTopic:     topic,
		Password:         password,
		ErrorMessage:     errorMsg,
		Updated:          updated,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Fehler beim Rendern der Seite", http.StatusInternalServerError)
		log.Println(err)
	}
}

// renderError renders an error message in the HTML template
func renderError(w http.ResponseWriter, errorMsg string) {
	renderTemplate(w, false, nil, 0, "", "", errorMsg, "")
}

// cleanupOldMeetings removes meeting data older than 6 hours
func cleanupOldMeetings() {
	for {
		time.Sleep(time.Hour)

		// Get list of account IDs without locking the password map for too long
		appState.PasswordMutex.RLock()
		accountIDs := make([]string, 0, len(appState.Meetings))
		for accountID := range appState.Meetings {
			accountIDs = append(accountIDs, accountID)
		}
		appState.PasswordMutex.RUnlock()

		for _, accountID := range accountIDs {
			accountMutex, exists := appState.AccountMutexes[accountID]
			if exists {
				accountMutex.Lock()
				meetings := appState.Meetings[accountID]
				for uuid, meeting := range meetings {
					if time.Since(meeting.LastUpdated) > 6*time.Hour {
						delete(meetings, uuid)
						log.Printf("Cleaned up old meeting: %s for account: %s", uuid, accountID)
					}
				}
				if len(meetings) == 0 {
					// Clean up empty account data
					delete(appState.Meetings, accountID)
					delete(appState.AccountMutexes, accountID)
					log.Printf("Removed empty account data for: %s", accountID)
				}
				accountMutex.Unlock()
			}
		}
	}
}

// SetupHandlers sets up the HTTP routes
func SetupHandlers(router *httprouter.Router, db *sql.DB) {
	appState.DB = db
	router.POST("/webhook", webhookHandler)
	router.GET("/", viewParticipantsHandler)
	router.POST("/", viewParticipantsHandler)
	router.POST("/add-account", addAccountHandler)
	router.GET("/test", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		renderTemplate(w, true, []string{
			"Alice Smith",
			"Bob Johnson",
			"Charlie Brown",
			"David Wilson",
			"Eve Davis",
			"Frank Miller",
			"Grace Lee",
			"Hannah Garcia",
			"Ian Martinez",
			"Jack Taylor",
			"Kate Anderson",
			"Liam Thomas",
			"Mia Jackson",
			"Noah White",
			"Olivia Harris",
			"Paul Clark",
			"Quinn Lewis",
			"Rachel Walker",
			"Sam Hall",
			"Tina Young",
			"Uma King",
			"Vera Wright",
			"Walter Scott",
			"Xander Green",
			"Yara Adams",
			"Zoe Baker",
		}, 26, "Simulated Demo", "", "", time.Now().Format("2006-01-02 15:04:05"))
	})

	// Start cleanup routine
	go cleanupOldMeetings()
}
