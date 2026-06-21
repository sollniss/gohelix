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

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		f.Close()
		return err
	}

	fmt.Println(string(buf[:n]))
	f.Close()
	return nil
}
