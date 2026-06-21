package stats

func sumSquares(nums []int) int {
	sum := 0
	for i := 0; i < len(nums); i++ {
		n := nums[i]
		sum += n * n
	}
	return sum
}
