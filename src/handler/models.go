package handler

import (
	"database/sql"
	"sync"
	"time"
)

// ZoomWebhookPayload represents the structure of incoming Zoom webhook events
type ZoomWebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		AccountID string `json:"account_id"`
		Object    struct {
			ID          string `json:"id"`
			UUID        string `json:"uuid"`
			Topic       string `json:"topic"`
			Participant struct {
				UserID   string `json:"user_id"`
				UserName string `json:"user_name"`
				Email    string `json:"email"`
			} `json:"participant"`
		} `json:"object"`
		PlainToken string `json:"plainToken"`
	} `json:"payload"`
}

// MeetingData holds participant data for a specific meeting
type MeetingData struct {
	Participants map[string]string // Key: UserID or Name, Value: Display Name
	Topic        string
	LastUpdated  time.Time
}

// AppState holds the application state with thread-safe access
type AppState struct {
	Meetings            map[string]map[string]*MeetingData // Key: AccountID -> Meeting UUID -> MeetingData
	AccountMutexes      map[string]*sync.RWMutex           // Key: AccountID -> Mutex for that account's meetings
	PasswordToAccountID map[string]string                  // Key: ViewerPassword -> AccountID
	PasswordMutex       sync.RWMutex                       // Dedicated mutex for password map
	DB                  *sql.DB
}
