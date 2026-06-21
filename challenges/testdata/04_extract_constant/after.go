package cache

const ringSize = 256

func newRing() ([]byte, []byte, int) {
	buf := make([]byte, ringSize)
	warm := make([]byte, 0, ringSize)
	mask := ringSize - 1
	return buf, warm, mask
}
