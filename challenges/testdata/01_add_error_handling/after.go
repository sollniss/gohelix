package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func loadConfig() (*Config, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}
	return &cfg, nil
}

type Config struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}
