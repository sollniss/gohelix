package main

type Config struct {
	Host  string `json:"host"`
	Port  int    `json:"port"`
	Name  string `json:"name"`
	Debug bool   `json:"debug"`
}
