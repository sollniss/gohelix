package job

import (
	"log"
	"os"
)

func run(path string) error {
	log.Printf("starting %s", path)
	info, err := os.Stat(path)
	if err != nil {
		log.Printf("stat failed: %v", err)
		return err
	}
	log.Printf("finished %d", info.Size())
	return nil
}
