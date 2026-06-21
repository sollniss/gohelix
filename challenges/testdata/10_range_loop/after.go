package stats

func sumSquares(nums []int) int {
	sum := 0
	for _, n := range nums {
		sum += n * n
	}
	return sum
}
