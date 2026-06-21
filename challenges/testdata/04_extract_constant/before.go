package cache

func newRing() ([]byte, []byte, int) {
	buf := make([]byte, 256)
	warm := make([]byte, 0, 256)
	mask := 256 - 1
	return buf, warm, mask
}
