package job

import (
	"fmt"
	"os"
)

func run(path string) error {
	fmt.Println("starting", path)
	info, err := os.Stat(path)
	if err != nil {
		fmt.Println("stat failed", err)
		return err
	}
	fmt.Println("finished", info.Size())
	return nil
}
