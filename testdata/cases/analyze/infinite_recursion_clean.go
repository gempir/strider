package analyze_cases

func recurseUntil(done bool) {
	if done {
		return
	}
	recurseUntil(done)
}

func recursivelySpawn() {
	go recursivelySpawn()
}
