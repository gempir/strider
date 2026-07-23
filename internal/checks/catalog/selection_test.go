//strider:ignore-file cognitive-complexity,confusing-naming,function-length
package catalog

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

type selectionCheck struct {
	meta Meta
}

func (check selectionCheck) Meta() Meta {
	return check.meta
}

func TestSelectContracts(t *testing.T) {
	maxLines := 25
	checks := []selectionCheck{
		{
			meta: Meta{
				Code:            "alpha-check",
				DefaultSeverity: diagnostic.SeverityWarning,
				Options: []Option{
					NonNegativeIntOption("max-lines", 10, "Maximum lines."),
				},
			},
		},
		{
			meta: Meta{
				Code:            "beta-check",
				DefaultSeverity: diagnostic.SeverityNote,
			},
		},
	}
	tests := []struct {
		name    string
		options SelectionOptions[selectionCheck]
		want    []string
		err     string
	}{
		{
			name: "defaults",
			options: SelectionOptions[selectionCheck]{
				Checks: checks,
			},
			want: []string{
				"alpha-check",
				"beta-check",
			},
		},
		{
			name: "case normalization and severity override",
			options: SelectionOptions[selectionCheck]{
				Checks: checks,
				Only: []string{
					"ALPHA-CHECK",
				},
				Settings: map[string]config.CheckConfig{
					"AlPhA-ChEcK": {
						Severity: string(diagnostic.SeverityError),
						Options: map[string]config.OptionValue{
							"max-lines": config.IntValue(maxLines),
						},
					},
				},
				MinimumSeverity: diagnostic.SeverityWarning,
			},
			want: []string{
				"alpha-check",
			},
		},
		{
			name: "minimum severity",
			options: SelectionOptions[selectionCheck]{
				Checks:          checks,
				MinimumSeverity: diagnostic.SeverityWarning,
			},
			want: []string{
				"alpha-check",
			},
		},
		{
			name: "duplicate descriptors",
			options: SelectionOptions[selectionCheck]{
				Checks: append(checks, selectionCheck{
					meta: Meta{
						Code:            "ALPHA-CHECK",
						DefaultSeverity: diagnostic.SeverityWarning,
					},
				}),
			},
			err: "duplicate check code",
		},
		{
			name: "unknown setting and selection are sorted",
			options: SelectionOptions[selectionCheck]{
				Checks: checks,
				Only: []string{
					"zeta-check",
				},
				Settings: map[string]config.CheckConfig{
					"gamma-check": {},
				},
			},
			err: "unknown check(s): gamma-check, zeta-check",
		},
		{
			name: "unsupported option",
			options: SelectionOptions[selectionCheck]{
				Checks: checks,
				Settings: map[string]config.CheckConfig{
					"beta-check": {
						Options: map[string]config.OptionValue{
							"max-lines": config.IntValue(maxLines),
						},
					},
				},
			},
			err: "does not support max-lines",
		},
		{
			name: "wrong option kind",
			options: SelectionOptions[selectionCheck]{
				Checks: checks,
				Settings: map[string]config.CheckConfig{
					"alpha-check": {
						Options: map[string]config.OptionValue{
							"max-lines": config.StringsValue([]string{
								"ten",
							}),
						},
					},
				},
			},
			err: "option max-lines must be int",
		},
		{
			name: "option range",
			options: SelectionOptions[selectionCheck]{
				Checks: checks,
				Settings: map[string]config.CheckConfig{
					"alpha-check": {
						Options: map[string]config.OptionValue{
							"max-lines": config.IntValue(-1),
						},
					},
				},
			},
			err: "option max-lines must be at least 0",
		},
		{
			name: "duplicate option spellings",
			options: SelectionOptions[selectionCheck]{
				Checks: checks,
				Settings: map[string]config.CheckConfig{
					"alpha-check": {
						Options: map[string]config.OptionValue{
							"MAX-LINES": config.IntValue(11),
							"max-lines": config.IntValue(12),
						},
					},
				},
			},
			err: `duplicate case-insensitive check option(s): "MAX-LINES", "max-lines"`,
		},
		{
			name: "invalid severity",
			options: SelectionOptions[selectionCheck]{
				Checks: checks,
				Settings: map[string]config.CheckConfig{
					"alpha-check": {
						Severity: "fatal",
					},
				},
			},
			err: "severity must be",
		},
	}
	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				selection, err := Select(test.options)
				if test.err != "" {
					if err == nil || !strings.Contains(err.Error(), test.err) {
						t.Fatalf("Select error = %v, want %q", err, test.err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				codes := make([]string, len(selection.Checks))
				for index, check := range selection.Checks {
					codes[index] = check.Meta().Code
				}
				if !reflect.DeepEqual(codes, test.want) {
					t.Fatalf("selected %v, want %v", codes, test.want)
				}
			},
		)
	}
}

func TestSelectOwnsNormalizedSettings(t *testing.T) {
	excludes := []string{
		"generated/**",
	}
	settings := map[string]config.CheckConfig{
		"EXAMPLE-CHECK": {
			Excludes: excludes,
		},
	}
	selection, err := Select(
		SelectionOptions[selectionCheck]{
			Checks: []selectionCheck{
				{
					meta: Meta{
						Code:            "example-check",
						DefaultSeverity: diagnostic.SeverityWarning,
					},
				},
			},
			Settings: settings,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	selected := selection.Settings["example-check"]
	selected.Excludes[0] = "changed/**"
	if settings["EXAMPLE-CHECK"].Excludes[0] != "generated/**" {
		t.Fatal("selection retained caller-owned option storage")
	}
	excludes[0] = "caller-change/**"
	if selection.Settings["example-check"].Excludes[0] != "changed/**" {
		t.Fatal("caller mutation changed selected settings")
	}
}
