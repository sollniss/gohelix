package example

import "fmt"

func Process(entries []string) {
	for i, entry := range entries {
		fmt.Printf("%d: %s\n", i, entry)
	}
	fmt.Printf("processed %d entries\n", len(entries))
}
