package semantic

import "testing"

func TestSlogArgumentShapeConservativeCases(t *testing.T) {
	assertCheckDiagnostics(
		t,
		"slog-argument-shape",
		`package sample

import "log/slog"

type key string

func check(value any) {
	slog.Info("odd", "key")
	slog.Info("non-string", 42, value)
	slog.Info("mixed", slog.String("key", "value"), "other", value)
	slog.Info("named string", key("key"), value)
	slog.Info("pairs", "key", value)
	slog.Info("attrs", slog.String("key", "value"), slog.Any("other", value))
}
`,
		4,
		"slog",
	)
}
