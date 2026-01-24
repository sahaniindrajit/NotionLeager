package main

import (
	"log"
	"net/http"
	"notionLeager/config"

	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env")
	}

	cfg := config.Load()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Println("Listening on port", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
