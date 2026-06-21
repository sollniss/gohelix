package main

import "fmt"

func greet(title, name string) string {
	return fmt.Sprintf("Hello, %s %s!", title, name)
}

func main() {
	greet("Ms", "Alice")
}
