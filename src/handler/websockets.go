package handler

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "http://localhost:8080" || origin == "https://zoom.8bj.de"
	},
}

type conndata struct {
	bool
	lastKeepalive time.Time
}

// Map of accountID to active WebSocket connections
var wsConnections = struct {
	sync.RWMutex
	conns map[string]map[*websocket.Conn]conndata
}{conns: make(map[string]map[*websocket.Conn]conndata)}

// Add or update connection with keepalive
func addConnection(accountID string, conn *websocket.Conn) {
	wsConnections.Lock()
	defer wsConnections.Unlock()
	if wsConnections.conns[accountID] == nil {
		wsConnections.conns[accountID] = make(map[*websocket.Conn]conndata)
	}
	wsConnections.conns[accountID][conn] = conndata{true, time.Now()}
}

// Remove connection
func removeConnection(accountID string, conn *websocket.Conn) {
	wsConnections.Lock()
	defer wsConnections.Unlock()
	if conns, ok := wsConnections.conns[accountID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(wsConnections.conns, accountID)
		}
	}
}

// Broadcast sorted participant list to connected clients for an account
func broadcastParticipants(accountID string, participants map[string]string) {
	names := make([]string, 0, len(participants))
	for _, name := range participants {
		names = append(names, name)
	}
	sort.Strings(names)
	data, err := json.Marshal(names)
	if err != nil {
		log.Printf("Error marshaling participants: %v", err)
		return
	}

	broadcastData(accountID, data)
}

// broadcastJoined broadcasts a single participant joined event
func broadcastJoined(accountID string, participantName string) {
	message := map[string]string{
		"action": "add",
		"name":   participantName,
	}
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling joined participant: %v", err)
		wsConnections.RUnlock()
		return
	}

	broadcastData(accountID, data)
}

// broadcastLeft broadcasts a single participant left event
func broadcastLeft(accountID string, participantName string) {
	message := map[string]string{
		"action": "remove",
		"name":   participantName,
	}
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling left participant: %v", err)
		wsConnections.RUnlock()
		return
	}

	broadcastData(accountID, data)
}

func broadcastData(accountID string, data []byte) {
	wsConnections.RLock()
	conns := wsConnections.conns[accountID]
	if conns == nil {
		wsConnections.RUnlock()
		return
	}

	var toRemove []*websocket.Conn
	now := time.Now()
	for conn, info := range conns {
		if now.Sub(info.lastKeepalive) > time.Minute {
			toRemove = append(toRemove, conn)
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error writing to websocket: %v", err)
			toRemove = append(toRemove, conn)
		} else {
			info.lastKeepalive = now
			conns[conn] = info
		}
	}
	wsConnections.RUnlock()

	wsConnections.Lock()
	for _, conn := range toRemove {
		conn.Close()
		delete(wsConnections.conns[accountID], conn)
	}
	if len(wsConnections.conns[accountID]) == 0 {
		delete(wsConnections.conns, accountID)
	}
	wsConnections.Unlock()
}

// WebSocket handler endpoint
func wsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	viewerPassword := r.URL.Query().Get("password")
	if viewerPassword == "" {
		http.Error(w, "Missing password", http.StatusUnauthorized)
		return
	}

	appState.PasswordMutex.RLock()
	accountID, ok := appState.PasswordToAccountID[viewerPassword]
	appState.PasswordMutex.RUnlock()
	if !ok {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	addConnection(accountID, conn)
	defer removeConnection(accountID, conn)

	sendCurrentParticipants(accountID, conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		wsConnections.Lock()
		if conns, ok := wsConnections.conns[accountID]; ok {
			if info, ok := conns[conn]; ok {
				info.lastKeepalive = time.Now()
				conns[conn] = info
			}
		}
		wsConnections.Unlock()
	}
}

// Helper to send current participants to a new connection
func sendCurrentParticipants(accountID string, conn *websocket.Conn) {
	ensureAccountInitialized(accountID)
	appState.AccountMutexes[accountID].RLock()
	meetings := appState.Meetings[accountID]
	var latestMeeting *MeetingData
	for _, meeting := range meetings {
		if latestMeeting == nil || meeting.LastUpdated.After(latestMeeting.LastUpdated) {
			latestMeeting = meeting
		}
	}
	var names []string
	if latestMeeting != nil {
		for _, name := range latestMeeting.Participants {
			names = append(names, name)
		}
		sort.Strings(names)
	}
	appState.AccountMutexes[accountID].RUnlock()

	message := map[string]interface{}{
		"action":       "reset",
		"participants": names,
	}
	data, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, data)
}
