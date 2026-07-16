package analyze_cases

func inlineMapKey(items map[string]int, bytes []byte) int {
	return items[string(bytes)]
}
