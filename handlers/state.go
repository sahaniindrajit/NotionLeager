package handlers

type EditState struct {
	PageID string
	Field  string
}

var editSessions = map[int64]*EditState{}
