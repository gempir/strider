package app

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/gempir/strider/internal/ui"
)

var commandOptionAliases = map[string]map[string]string{
	"strider": {
		"config":    "c",
		"no-config": "n",
		"color":     "C",
		"colors":    "C",
		"help":      "h",
		"version":   "v",
	},
	"fmt": {
		"check":          "c",
		"diff":           "d",
		"write":          "w",
		"stdin":          "s",
		"stdin-filename": "f",
		"help":           "h",
	},
	"check": {
		"format":                           "f",
		"minimum-severity":                 "s",
		"summary-only":                     "q",
		"watch":                            "w",
		"list-checks":                      "l",
		"list-rules":                       "l",
		"explain":                          "e",
		"baseline":                         "b",
		"generate-baseline":                "g",
		"remove-outdated-baseline-entries": "r",
		"only":                             "o",
		"fix":                              "x",
		"fix-unsafe":                       "u",
		"help":                             "h",
	},
}

type globalOptions struct {
	configPath string
	noConfig   bool
	color      string
	colorSet   bool
}

type stringList []string

func parseGlobalOptions(args []string, stderr io.Writer) ([]string, globalOptions, bool) {
	options := globalOptions{}
	aliases := commandOptionAliases["strider"]
	for len(args) != 0 {
		switch {
		case args[0] == "--config" || args[0] == "-c":
			if len(args) < 2 || args[1] == "" {
				printCommandError(stderr, globalColor(options), "strider", "--config requires a path")
				return nil, globalOptions{}, false
			}
			options.configPath = args[1]
			args = args[2:]
		case strings.HasPrefix(args[0], "--config="):
			options.configPath = strings.TrimPrefix(args[0], "--config=")
			if options.configPath == "" {
				printCommandError(stderr, globalColor(options), "strider", "--config requires a path")
				return nil, globalOptions{}, false
			}
			args = args[1:]
		case strings.HasPrefix(args[0], "-c="):
			options.configPath = strings.TrimPrefix(args[0], "-c=")
			if options.configPath == "" {
				printCommandError(stderr, globalColor(options), "strider", "--config requires a path")
				return nil, globalOptions{}, false
			}
			args = args[1:]
		case args[0] == "--no-config" || args[0] == "-n":
			options.noConfig = true
			args = args[1:]
		case args[0] == "--color" || args[0] == "--colors" || args[0] == "-C":
			if len(args) < 2 || !ui.ValidColorMode(args[1]) {
				printCommandError(stderr, globalColor(options), "strider", "--color must be auto, always, or never")
				return nil, globalOptions{}, false
			}
			options.color = args[1]
			options.colorSet = true
			args = args[2:]
		case strings.HasPrefix(args[0], "--color=") || strings.HasPrefix(args[0], "--colors="):
			_, value, _ := strings.Cut(args[0], "=")
			if !ui.ValidColorMode(value) {
				printCommandError(stderr, globalColor(options), "strider", "--color must be auto, always, or never")
				return nil, globalOptions{}, false
			}
			options.color = value
			options.colorSet = true
			args = args[1:]
		case strings.HasPrefix(args[0], "-C="):
			value := strings.TrimPrefix(args[0], "-C=")
			if !ui.ValidColorMode(value) {
				printCommandError(stderr, globalColor(options), "strider", "--color must be auto, always, or never")
				return nil, globalOptions{}, false
			}
			options.color = value
			options.colorSet = true
			args = args[1:]
		case strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[0], "--") && len(args[0]) > 2:
			name := strings.TrimPrefix(strings.SplitN(args[0], "=", 2)[0], "-")
			replacement := "--" + name
			if short := aliases[name]; short != "" {
				replacement += " or -" + short
			}
			printCommandError(stderr, globalColor(options), "strider", "long option %q must use two dashes; use %s", args[0], replacement)
			return nil, globalOptions{}, false
		default:
			return validateGlobalOptions(args, options, stderr)
		}
	}
	return validateGlobalOptions(args, options, stderr)
}

func validateGlobalOptions(args []string, options globalOptions, stderr io.Writer) ([]string, globalOptions, bool) {
	if options.configPath != "" && options.noConfig {
		printCommandError(stderr, globalColor(options), "strider", "--config and --no-config are mutually exclusive")
		return nil, globalOptions{}, false
	}
	return args, options, true
}

func globalColor(options globalOptions) ui.ColorMode {
	if options.colorSet {
		return ui.ColorMode(options.color)
	}
	return ui.ColorAuto
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(current *flag.Flag) {
		if current.Name == name {
			found = true
		}
	})
	return found
}

func flagWasSetAny(flags *flag.FlagSet, names ...string) bool {
	for _, name := range names {
		if flagWasSet(flags, name) {
			return true
		}
	}
	return false
}

func stringOption(flags *flag.FlagSet, long, short, fallback, usage string) *string {
	value := fallback
	flags.StringVar(&value, long, fallback, usage)
	flags.StringVar(&value, short, fallback, "alias for --"+long)
	return &value
}

func boolOption(flags *flag.FlagSet, long, short, usage string) *bool {
	value := false
	flags.BoolVar(&value, long, false, usage)
	flags.BoolVar(&value, short, false, "alias for --"+long)
	return &value
}

func varOption(flags *flag.FlagSet, value flag.Value, long, short, usage string) {
	flags.Var(value, long, usage)
	flags.Var(value, short, "alias for --"+long)
}

func parseCommandFlags(flags *flag.FlagSet, args []string, aliases map[string]string, command string, colorMode ui.ColorMode, stderr io.Writer) bool {
	for _, argument := range args {
		if argument == "--" {
			break
		}
		if !strings.HasPrefix(argument, "-") || strings.HasPrefix(argument, "--") || len(argument) <= 2 || isShortOptionAssignment(argument, aliases) {
			continue
		}
		name := strings.TrimPrefix(strings.SplitN(argument, "=", 2)[0], "-")
		short := aliases[name]
		replacement := "--" + name
		if short != "" {
			replacement += " or -" + short
		}
		printCommandError(stderr, colorMode, "strider "+command, "long option %q must use two dashes; use %s", argument, replacement)
		return false
	}
	return flags.Parse(args) == nil
}

func isShortOptionAssignment(argument string, aliases map[string]string) bool {
	if len(argument) < 4 || argument[0] != '-' || argument[2] != '=' {
		return false
	}
	short := argument[1:2]
	for _, alias := range aliases {
		if alias == short {
			return true
		}
	}
	return false
}

func printFlagDefaults(writer io.Writer, flags *flag.FlagSet, aliases map[string]string, palette ui.Palette) {
	flags.VisitAll(
		func(option *flag.Flag) {
			if len(option.Name) == 1 {
				return
			}
			if option.Name == "list-rules" {
				return
			}
			value := " VALUE"
			if boolean, ok := option.Value.(interface {
				IsBoolFlag() bool
			}); ok && boolean.IsBoolFlag() {
				value = ""
			}
			short := "    "
			if alias := aliases[option.Name]; alias != "" {
				short = "-" + alias + ", "
			}
			usage := option.Usage
			if option.DefValue != "" && option.DefValue != "false" {
				usage += fmt.Sprintf(" (default %q)", option.DefValue)
			}
			fmt.Fprintf(writer, "  %s%s%s\n      %s\n", palette.Code(short), palette.Code("--"+option.Name), palette.Muted(value), usage)
		},
	)
}

func (values *stringList) String() string {
	return strings.Join(*values, ",")
}

func (values *stringList) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			*values = append(*values, item)
		}
	}
	return nil
}
