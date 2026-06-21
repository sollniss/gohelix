package report

import "strconv"

func line(name string, count int, ok bool) string {
	return name + ": " + strconv.Itoa(count) + " items (ok=" + strconv.FormatBool(ok) + ")"
}
