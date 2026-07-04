package report

import "fmt"

func Line(name string, count int, ok bool) string {
	return fmt.Sprintf("%s: %d items (ok=%t)", name, count, ok)
}
