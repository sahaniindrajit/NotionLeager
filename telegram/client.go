package telegram

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func SendMessage(token string, chatID int64, text string) error {

	body, _ := json.Marshal(SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	})

	_, err := http.Post(
		"https://api.telegram.org/bot"+token+"/sendMessage",
		"application/json",
		bytes.NewBuffer(body),
	)

	return err
}
