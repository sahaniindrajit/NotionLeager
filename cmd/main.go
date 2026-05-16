package main

import (
	"log"
	"net/http"
	"notionLeager/config"
	"notionLeager/handlers"
	"notionLeager/utils"

	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env")
	}

	cfg := config.Load()
	utils.InitLogger(cfg.Env)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/webhook", handlers.TelegramWebhook(*cfg))

	log.Println("Listening on port", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
