package analyze_cases

func effectiveBitwiseOperation(value, mask uint) uint {
	shifted := value << 0
	return shifted & mask
}
