// Package config loads and validates Strider's project configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/ui"
)

const Filename = "strider.toml"

var behavioralCheckOptions = []string{
	"characters",
	"max-lines",
	"max-statements",
	"max-results",
	"max-parameters",
	"max-public-structs",
	"max-methods",
	"blocked-imports",
}

// Config is the complete project configuration document.
type Config struct {
	Version   int             `toml:"version"`
	Color     string          `toml:"color"`
	Formatter FormatterConfig `toml:"formatter"`
	Checks    ToolConfig      `toml:"-"`

	Path string `toml:"-"`
	Root string `toml:"-"`
}

type FormatterConfig struct {
	PrintWidth int      `toml:"print-width"`
	Excludes   []string `toml:"excludes"`
}

type ToolConfig struct {
	Excludes        []string               `toml:"excludes"`
	Baseline        string                 `toml:"baseline"`
	MinimumSeverity string                 `toml:"minimum-severity"`
	Settings        map[string]CheckConfig `toml:"-"`
}

type fileConfig struct {
	Version   int                       `toml:"version"`
	Color     string                    `toml:"color"`
	Formatter FormatterConfig           `toml:"formatter"`
	Check     map[string]toml.Primitive `toml:"check"`
	Checks    map[string]toml.Primitive `toml:"checks"`
}

type CheckConfig struct {
	Severity         string   `toml:"severity"`
	Excludes         []string `toml:"excludes"`
	Characters       []string `toml:"characters"`
	MaxLines         int      `toml:"max-lines"`
	MaxStatements    int      `toml:"max-statements"`
	MaxResults       int      `toml:"max-results"`
	MaxParameters    int      `toml:"max-parameters"`
	MaxPublicStructs int      `toml:"max-public-structs"`
	MaxMethods       int      `toml:"max-methods"`
	BlockedImports   []string `toml:"blocked-imports"`
	definedOptions   map[string]bool
}

func Defaults() Config {
	return Config{
		Version: 1,
		Color:   string(ui.ColorAuto),
		Formatter: FormatterConfig{
			PrintWidth: 180,
		},
		Checks: defaultToolConfig(),
	}
}

func defaultToolConfig() ToolConfig {
	return ToolConfig{
		MinimumSeverity: string(diagnostic.SeverityWarning),
		Settings:        make(map[string]CheckConfig),
	}
}

// Load reads an explicit path or discovers strider.toml from the working
// directory upward. With no discovered file it returns built-in defaults.
func Load(explicitPath string, disabled bool) (Config, error) {
	configuration := Defaults()
	if disabled {
		return configuration, nil
	}
	path := explicitPath
	if path == "" {
		var err error
		path, err = discover()
		if err != nil {
			return Config{}, err
		}
		if path == "" {
			return configuration, nil
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Config{}, fmt.Errorf("configuration path: %w", err)
	}
	if canonical, canonicalErr := filepath.EvalSymlinks(absolute); canonicalErr == nil {
		absolute = canonical
	}
	decoded := fileConfig{
		Version:   configuration.Version,
		Color:     configuration.Color,
		Formatter: configuration.Formatter,
	}
	metadata, err := toml.DecodeFile(absolute, &decoded)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", absolute, err)
	}
	configuration.Version = decoded.Version
	configuration.Color = decoded.Color
	configuration.Formatter = decoded.Formatter
	if err := decodeCheck(&configuration.Checks, decoded.Check, metadata); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", absolute, err)
	}
	if err := decodeChecks(&configuration.Checks, decoded.Checks, metadata); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", absolute, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		return Config{}, fmt.Errorf("%s: unknown configuration key(s): %s", absolute, strings.Join(keys, ", "))
	}
	recordDefinedCheckOptions(&configuration, metadata)
	configuration.Path = absolute
	configuration.Root = filepath.Dir(absolute)
	if err := configuration.validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", absolute, err)
	}
	return configuration, nil
}

func recordDefinedCheckOptions(configuration *Config, metadata toml.MetaData) {
	for code, check := range configuration.Checks.Settings {
		for _, option := range behavioralCheckOptions {
			if !metadata.IsDefined("checks", code, option) {
				continue
			}
			if check.definedOptions == nil {
				check.definedOptions = make(map[string]bool)
			}
			check.definedOptions[option] = true
		}
		configuration.Checks.Settings[code] = check
	}
}

func decodeCheck(destination *ToolConfig, values map[string]toml.Primitive, metadata toml.MetaData) error {
	for name, value := range values {
		switch name {
		case "excludes":
			if err := metadata.PrimitiveDecode(value, &destination.Excludes); err != nil {
				return err
			}
		case "baseline":
			if err := metadata.PrimitiveDecode(value, &destination.Baseline); err != nil {
				return err
			}
		case "minimum-severity":
			if err := metadata.PrimitiveDecode(value, &destination.MinimumSeverity); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown configuration key(s): check.%s", name)
		}
	}
	return nil
}

func decodeChecks(destination *ToolConfig, values map[string]toml.Primitive, metadata toml.MetaData) error {
	for code, value := range values {
		if metadata.Type("checks", code) != "Hash" {
			return fmt.Errorf("unknown configuration key(s): checks.%s", code)
		}
		check := CheckConfig{}
		if err := metadata.PrimitiveDecode(value, &check); err != nil {
			return err
		}
		destination.Settings[code] = check
	}
	return nil
}

// HasExplicitOption reports whether a behavioral option was present in the
// decoded TOML, including when its value decoded to the Go zero value.
func (check CheckConfig) HasExplicitOption(option string) bool {
	return check.definedOptions[option]
}

func discover() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(directory, Filename)
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", nil
		}
		directory = parent
	}
}

func (configuration Config) validate() error {
	if configuration.Version != 1 {
		return fmt.Errorf("unsupported configuration version %d; expected 1", configuration.Version)
	}
	if !ui.ValidColorMode(configuration.Color) {
		return fmt.Errorf("color must be auto, always, or never")
	}
	if configuration.Formatter.PrintWidth < 40 || configuration.Formatter.PrintWidth > 500 {
		return fmt.Errorf("formatter.print-width must be between 40 and 500")
	}
	return validateTool("check", configuration.Checks)
}

func validateTool(name string, tool ToolConfig) error {
	if !diagnostic.ValidSeverity(diagnostic.Severity(tool.MinimumSeverity)) {
		return fmt.Errorf("%s.minimum-severity must be none, note, warning, or error", name)
	}
	for code, check := range tool.Settings {
		if check.Severity != "" && !diagnostic.ValidSeverity(diagnostic.Severity(check.Severity)) {
			return fmt.Errorf("%s.%s.severity must be none, note, warning, or error", name, code)
		}
		for option, value := range map[string]int{
			"max-lines":          check.MaxLines,
			"max-statements":     check.MaxStatements,
			"max-results":        check.MaxResults,
			"max-parameters":     check.MaxParameters,
			"max-public-structs": check.MaxPublicStructs,
			"max-methods":        check.MaxMethods,
		} {
			if value < 0 {
				return fmt.Errorf("%s.%s.%s must not be negative", name, code, option)
			}
		}
	}
	return nil
}

func (configuration Config) Resolve(path string) string {
	if path == "" || filepath.IsAbs(path) || configuration.Root == "" {
		return path
	}
	return filepath.Join(configuration.Root, filepath.FromSlash(path))
}

// EffectiveCheck returns one check setting with the tool-wide exclusions
// appended.
func (configuration Config) EffectiveCheck(code string) CheckConfig {
	check := cloneCheckConfig(configuration.Checks.Settings[code])
	check.Excludes = append(append([]string(nil), configuration.Checks.Excludes...), check.Excludes...)
	return check
}

func cloneCheckConfig(check CheckConfig) CheckConfig {
	cloned := check
	cloned.Excludes = append([]string(nil), check.Excludes...)
	cloned.Characters = append([]string(nil), check.Characters...)
	cloned.BlockedImports = append([]string(nil), check.BlockedImports...)
	if check.definedOptions != nil {
		cloned.definedOptions = make(map[string]bool, len(check.definedOptions))
		for option := range check.definedOptions {
			cloned.definedOptions[option] = true
		}
	}
	return cloned
}
