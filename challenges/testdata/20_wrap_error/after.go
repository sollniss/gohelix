package example

import (
	"fmt"
	"os"
)

func readConfig(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readConfig %s: %w", path, err)
	}
	return data, nil
}
