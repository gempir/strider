package semantic

import "testing"

func TestTestMainMissingExitReportsLegacyModules(t *testing.T) {
	root := analysisModuleVersion(t, "1.14", `package sample

import "testing"

func TestMain(m *testing.M) {
	m.Run()
}
`)
	registry, err := newRegistry([]string{
		"test-main-missing-exit",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
}
