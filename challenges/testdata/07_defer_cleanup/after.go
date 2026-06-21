package main

import (
	"fmt"
	"os"
)

func processFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		return err
	}

	fmt.Println(string(buf[:n]))
	return nil
}
