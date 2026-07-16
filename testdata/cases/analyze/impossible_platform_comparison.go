//go:build darwin

package analyze_cases

import "runtime"

func impossiblePlatformComparison() bool {
	return runtime.GOOS == "windows"
}
