package analyze_cases

func allocatedMapKey(items map[string]int, bytes []byte) int {
	key := string(bytes)
	return items[key]
}
