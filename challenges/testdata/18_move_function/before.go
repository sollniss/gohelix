package main

import "fmt"

func main() {
	fmt.Println(add(1, 2))
	fmt.Println(multiply(3, 4))
}

func add(a, b int) int {
	return a + b
}

func multiply(a, b int) int {
	return a * b
}
