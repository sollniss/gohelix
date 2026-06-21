package config

import (
	"errors"
	"strings"
)

func parse(line string) (string, error) {
	if !strings.Contains(line, "=") {
		return "", errors.New("missing separator")
	}
	key, _, _ := strings.Cut(line, "=")
	return strings.TrimSpace(key), nil
}
