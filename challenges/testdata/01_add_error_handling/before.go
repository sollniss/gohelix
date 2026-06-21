package main

import (
	"encoding/json"
	"os"
)

func loadConfig() *Config {
	data, _ := os.ReadFile("config.json")
	var cfg Config
	json.Unmarshal(data, &cfg)
	return &cfg
}

type Config struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}
