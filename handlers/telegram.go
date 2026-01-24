package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"notionLeager/config"
	"notionLeager/expense"
	"notionLeager/telegram"
	"strconv"
	"time"
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

var deduper = expense.NewDeduper(30 * time.Second)

func TelegramWebhook(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

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

		ownerID, _ := strconv.ParseInt(cfg.TelegramOwnerId, 10, 64)
		if update.Message.From.ID != ownerID {
			return
		}

		if update.Message.From.ID != ownerID {
			w.WriteHeader(http.StatusOK)
			return
		}

		text := update.Message.Text
		key := fmt.Sprintf("%d:%s", update.Message.Chat.ID, text)
		if deduper.Seen(key) {
			return
		}
		log.Println("Owner message", text)

		if text == "/start" {
			telegram.SendMessage(
				cfg.TelegramBotToken,
				update.Message.Chat.ID,
				"👋 Hi!\n\nSend expenses like:\nLunch, 450, Food\nLunch, 450, Food, Office lunch\n\nCurrency: INR",
			)
			return
		}

		exp, err := expense.Parse(text)
		if err != nil {
			telegram.SendMessage(
				cfg.TelegramBotToken,
				update.Message.Chat.ID,
				"❌ Invalid format\n\nUse:\nName, Amount, Category, Description (optional)",
			)
			return
		}

		log.Printf("Parsed expense: %+v\n", exp)

		w.WriteHeader(http.StatusOK)
	}
}
