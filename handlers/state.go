package handlers

import (
	"sync"
	"time"
)

type EditState struct {
	PageID    string
	Field     string
	CreatedAt time.Time
}

var (
	editSessions   = make(map[int64]*EditState)
	editSessionsMu sync.RWMutex
	sessionTTL     = 5 * time.Minute
)

func init() {
	// Start cleanup goroutine
	go cleanupStaleSessions()
}

func cleanupStaleSessions() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		editSessionsMu.Lock()
		now := time.Now()
		for chatID, state := range editSessions {
			if now.Sub(state.CreatedAt) > sessionTTL {
				delete(editSessions, chatID)
			}
		}
		editSessionsMu.Unlock()
	}
}

func getEditSession(chatID int64) (*EditState, bool) {
	editSessionsMu.RLock()
	defer editSessionsMu.RUnlock()
	state, ok := editSessions[chatID]
	return state, ok
}

func setEditSession(chatID int64, state *EditState) {
	editSessionsMu.Lock()
	defer editSessionsMu.Unlock()
	state.CreatedAt = time.Now()
	editSessions[chatID] = state
}

func deleteEditSession(chatID int64) {
	editSessionsMu.Lock()
	defer editSessionsMu.Unlock()
	delete(editSessions, chatID)
}
