package main

func describe(x int) string {
	if x < 0 {
		return "negative"
	} else if x == 0 {
		return "zero"
	} else if x < 10 {
		return "small"
	} else {
		return "large"
	}
}
