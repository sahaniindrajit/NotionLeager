package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
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

func SendPhoto(token string, chatID int64, imagePath string, caption string) error {
	url := "https://api.telegram.org/bot" + token + "/sendPhoto"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("chat_id", fmt.Sprint(chatID))
	if caption != "" {
		_ = writer.WriteField("caption", caption)
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("photo", "chart.png")
	if err != nil {
		return err
	}

	_, _ = io.Copy(part, file)
	writer.Close()

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
