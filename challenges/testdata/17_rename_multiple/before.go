package example

import "fmt"

func process(items []string) {
	for i, item := range items {
		fmt.Printf("%d: %s\n", i, item)
	}
	fmt.Printf("processed %d items\n", len(items))
}
