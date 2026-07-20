package semantic

import (
	"strings"
	"testing"
)

func TestRangeValueCaptureRespectsGoVersionAssignmentAndInvocation(t *testing.T) {
	tests := []struct {
		name      string
		goVersion string
		source    string
		want      int
	}{
		{
			name:      "legacy declaration is reused",
			goVersion: "1.21",
			source: `package sample

func capture(values []int) []func() {
	var callbacks []func()
	for _, value := range values {
		callbacks = append(callbacks, func() { _ = value })
	}
	return callbacks
}
`,
			want: 1,
		},
		{
			name:      "modern declaration is per iteration",
			goVersion: "1.22",
			source: `package sample

func capture(values []int) []func() {
	var callbacks []func()
	for _, value := range values {
		callbacks = append(callbacks, func() { _ = value })
	}
	return callbacks
}
`,
		},
		{
			name:      "assignment remains reused",
			goVersion: "1.26",
			source: `package sample

func capture(values []int) []func() {
	var value int
	var callbacks []func()
	for _, value = range values {
		callbacks = append(callbacks, func() { _ = value })
	}
	return callbacks
}
`,
			want: 1,
		},
		{
			name:      "synchronous invocation is safe",
			goVersion: "1.21",
			source: `package sample

func use(int) {}

func capture(values []int) {
	for _, value := range values {
		func() { use(value) }()
	}
}
`,
		},
		{
			name:      "go invocation can outlive iteration",
			goVersion: "1.21",
			source: `package sample

func use(int) {}

func capture(values []int) {
	for _, value := range values {
		go func() { use(value) }()
	}
}
`,
			want: 1,
		},
	}
	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				root := analysisModuleVersion(t, test.goVersion, test.source)
				registry,
					err := newRegistry([]string{
					"range-value-capture",
				})
				if err != nil {
					t.Fatal(err)
				}
				diagnostics,
					err := Run([]string{
					root,
				}, registry)
				if err != nil {
					t.Fatal(err)
				}
				if len(diagnostics) != test.want {
					t.Fatalf("got %d diagnostics, want %d: %#v", len(diagnostics), test.want, diagnostics)
				}
				for _, item := range diagnostics {
					if item.Code != "range-value-capture" || !strings.Contains(item.Message, "range variable") {
						t.Fatalf("unexpected diagnostic: %#v", item)
					}
				}
			},
		)
	}
}
