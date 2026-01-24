package config

import (
	"log"
	"os"
)

type Config struct {
	Port             string
	Env              string
	TelegramOwnerId  string
	TelegramBotToken string
	NotionAPIKey     string
	NotionExpenseDB  string
}

func Load() *Config {

	cfg := &Config{
		Port:             os.Getenv("PORT"),
		Env:              os.Getenv("ENV"),
		TelegramOwnerId:  os.Getenv("TELEGRAM_OWNER_ID"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		NotionAPIKey:     os.Getenv("NOTION_API_KEY"),
		NotionExpenseDB:  os.Getenv("NOTION_EXPENSE_DB_ID"),
	}

	if cfg.Port == "" {
		log.Fatal("Port required")
	}
	if cfg.Env == "" {
		log.Fatal("Enviroment required")
	}
	if cfg.TelegramOwnerId == "" {
		log.Fatal("Telgram owner id required")
	}
	if cfg.TelegramBotToken == "" {
		log.Fatal("Telgram bot token required")
	}
	if cfg.NotionAPIKey == "" {
		log.Fatal("Notion API key required")
	}
	if cfg.NotionExpenseDB == "" {
		log.Fatal("Notion Expense db id  required")
	}

	return cfg
}
