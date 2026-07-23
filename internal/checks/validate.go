package checks

import "github.com/gempir/strider/internal/checks/semantic"

// ValidateOverlay parses and type-checks packages containing changed source.
// It keeps package-loading details behind the unified check boundary.
func ValidateOverlay(paths []string, overlay map[string][]byte) error {
	return semantic.ValidateOverlay(paths, overlay)
}
