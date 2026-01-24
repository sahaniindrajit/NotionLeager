package config

import (
	"log"
	"os"
)

type Config struct {
	Port string
	Env string
}

func Load() *Config {

	cfg := &Config{
		Port: os.Getenv("PORT"),
		Env: os.Getenv("ENV"),
	}

	if cfg.Port == ""{
		log.Fatal("Port required")
	}
	if cfg.Env == ""{
		log.Fatal("Enviroment required")

	}

	return cfg
}