//strider:ignore-file cognitive-complexity,cyclomatic-complexity
package syntax

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/config"
)

func TestConcreteCallChecks(t *testing.T) {
	fixture := writeFixture(
		t,
		`package sample

func helper() {
	runtime.GC()
	_ = errors.New(fmt.Sprintf("value %d", 1))
	_ = fmt.Errorf("static message")
	fmt.Printf("static message")
	print("message")
	sort.Slice(nil, nil)
	os.Exit(1)
	_ = errors.New("Bad message.")
}
`,
	)
	registry, err := NewRegistry(
		[]string{
			"call-to-gc",
			"deep-exit",
			"error-strings",
			"prefer-fmt-errorf",
			"unnecessary-format",
			"use-errors-new",
			"use-fmt-print",
			"use-slices-sort",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, item := range diagnostics {
		counts[item.Code]++
	}
	wanted := map[string]int{
		"call-to-gc":         1,
		"deep-exit":          1,
		"error-strings":      1,
		"prefer-fmt-errorf":  1,
		"unnecessary-format": 1,
		"use-errors-new":     1,
		"use-fmt-print":      1,
		"use-slices-sort":    1,
	}
	for code, count := range wanted {
		if counts[code] != count {
			t.Errorf("%s produced %d findings; want %d: %#v", code, counts[code], count, diagnostics)
		}
	}
}

func TestConcreteImportChecks(t *testing.T) {
	fixture := writeFixture(t, `package sample

import (
	. "fmt"
	_ "net/http"
	Bad_Alias "strings"
	strings "strings"
)
`)
	registry, err := NewRegistry([]string{
		"blank-imports",
		"dot-imports",
		"duplicated-imports",
		"import-alias-naming",
		"redundant-import-alias",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, item := range diagnostics {
		counts[item.Code]++
	}
	for _, code := range []string{
		"blank-imports",
		"dot-imports",
		"duplicated-imports",
		"import-alias-naming",
		"redundant-import-alias",
	} {
		if counts[code] != 1 {
			t.Errorf("%s produced %d findings; want 1: %#v", code, counts[code], diagnostics)
		}
	}
}

func TestExternalTestPackageMatchesNamingAndDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "sample")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "sample_test.go")
	if err := os.WriteFile(filename, []byte("package sample_test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry([]string{
		"package-naming",
		"package-directory-mismatch",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		filename,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("standard external test package produced diagnostics: %#v", diagnostics)
	}
}

func TestFileLengthLimitDefaultsTo500AndExplicitZeroDisables(t *testing.T) {
	fixture := writeFixture(t, "package sample\n"+strings.Repeat("// line\n", 500))
	defaultRegistry, err := NewRegistry([]string{
		"file-length-limit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := defaultRegistry.settings["file-length-limit"].options.Int("max-lines"); got != 500 {
		t.Fatalf("default max lines = %d, want 500", got)
	}
	diagnostics, err := Run([]string{
		fixture,
	}, defaultRegistry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
	configuredRegistry, err := NewRegistryWithOptions(
		RegistryOptions{
			Only: []string{
				"file-length-limit",
			},
			Settings: map[string]config.CheckConfig{
				"file-length-limit": {
					Options: map[string]config.OptionValue{
						"max-lines": config.IntValue(12),
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := configuredRegistry.settings["file-length-limit"].options.Int("max-lines"); got != 12 {
		t.Fatalf("configured max lines = %d, want 12", got)
	}

	configurationPath := filepath.Join(t.TempDir(), config.Filename)
	contents := "version = 1\n[checks.file-length-limit]\nmax-lines = 0\n"
	if err := os.WriteFile(configurationPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := config.Load(filepath.Dir(configurationPath), configurationPath, false)
	if err != nil {
		t.Fatal(err)
	}
	setting := configuration.Checks.Settings["file-length-limit"]
	disabledRegistry, err := NewRegistryConfigured([]string{
		"file-length-limit",
	}, map[string]config.CheckConfig{
		"file-length-limit": setting,
	}, configuration.Root)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := disabledRegistry.settings["file-length-limit"].options.Int("max-lines"); got != 0 {
		t.Fatalf("explicit max lines = %d, want disabled value 0", got)
	}
	diagnostics, err = Run([]string{
		fixture,
	}, disabledRegistry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("explicit max-lines = 0 produced diagnostics: %#v", diagnostics)
	}
}
