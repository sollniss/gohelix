package example

import (
	"fmt"
	"os"
)

func ReadConfig(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ReadConfig %s: %w", path, err)
	}
	return data, nil
}
