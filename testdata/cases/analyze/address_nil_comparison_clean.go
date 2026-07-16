package analyze_cases

func pointerNilComparison(pointer *int) bool {
	return pointer == nil
}

func dereferenceAddressNilComparison(pointer *int) bool {
	return &*pointer == nil
}
