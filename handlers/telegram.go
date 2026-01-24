package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int    `json:"message_id"`
		Text      string `json:"text"`
		From      struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

func TelegramWebhook(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var update TelegramUpdate

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Println("Invalid telegram update:", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if update.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf(
		"Incoming message: text=%q from=%d chat=%d",
		update.Message.Text,
		update.Message.From.ID,
		update.Message.Chat.ID,
	)

	w.WriteHeader(http.StatusOK)
}
