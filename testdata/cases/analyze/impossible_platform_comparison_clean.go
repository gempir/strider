//go:build darwin

package analyze_cases

import "runtime"

func possiblePlatformComparison() bool {
	return runtime.GOOS == "darwin"
}
