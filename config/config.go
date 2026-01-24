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
}

func Load() *Config {

	cfg := &Config{
		Port:             os.Getenv("PORT"),
		Env:              os.Getenv("ENV"),
		TelegramOwnerId:  os.Getenv("TELEGRAM_OWNER_ID"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
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

	return cfg
}
