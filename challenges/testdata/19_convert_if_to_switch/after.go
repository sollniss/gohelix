package main

func Describe(x int) string {
	switch {
	case x < 0:
		return "negative"
	case x == 0:
		return "zero"
	case x < 10:
		return "small"
	default:
		return "large"
	}
}
