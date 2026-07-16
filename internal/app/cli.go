package app

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/analyze"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/lint"
	"github.com/gempir/strider/internal/ui"
	"github.com/spf13/cobra"
)

type commandState struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	globals globalOptions
	code    int
}

type lintOptions struct {
	format          string
	listRules       bool
	allRules        bool
	explain         string
	baselinePath    string
	baselineVariant string
	generate        bool
	prune           bool
	ignore          bool
	backup          bool
	only            stringList
	paths           []string
}

type analyzeOptions struct {
	format          string
	listRules       bool
	explain         string
	baselinePath    string
	baselineVariant string
	generate        bool
	prune           bool
	ignore          bool
	backup          bool
	only            stringList
	paths           []string
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	state := &commandState{
		stdin: stdin, stdout: stdout, stderr: stderr, code: exitSuccess,
	}
	root := newRootCommand(state)
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	command, err := root.ExecuteC()
	if err == nil {
		return state.code
	}
	colorMode := ui.ColorAuto
	if ui.ValidColorMode(state.globals.color) {
		colorMode = ui.ColorMode(state.globals.color)
	}
	name := "strider"
	if command != nil {
		name = command.CommandPath()
	}
	printCommandError(stderr, colorMode, name, "%v", err)
	return exitError
}

func newRootCommand(state *commandState) *cobra.Command {
	var root *cobra.Command
	root = &cobra.Command{
		Use:           "strider",
		Short:         "Format, lint, and statically analyze Go code",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version,
		Run: func(command *cobra.Command, _ []string) {
			command.SetOut(state.stderr)
			_ = command.Help()
			command.SetOut(state.stdout)
			state.code = exitError
		},
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			state.globals.colorSet = root.PersistentFlags().Changed("color") ||
				root.PersistentFlags().Changed("colors")
			if root.PersistentFlags().Changed("config") && state.globals.configPath == "" {
				return fmt.Errorf("--config requires a path")
			}
			if state.globals.colorSet && !ui.ValidColorMode(state.globals.color) {
				return fmt.Errorf("--color must be auto, always, or never")
			}
			return nil
		},
	}
	root.SetVersionTemplate("strider {{.Version}}\n")

	flags := root.PersistentFlags()
	flags.StringVar(&state.globals.configPath, "config", "", "use this strider.toml instead of automatic discovery")
	flags.BoolVar(&state.globals.noConfig, "no-config", false, "disable configuration discovery")
	flags.StringVar(&state.globals.color, "color", "auto", "color output: auto, always, or never")
	flags.StringVar(&state.globals.color, "colors", "auto", "alias for --color")
	root.MarkFlagsMutuallyExclusive("config", "no-config")
	must(root.MarkPersistentFlagFilename("config", "toml"))
	must(root.RegisterFlagCompletionFunc(
		"color",
		cobra.FixedCompletions(
			[]cobra.Completion{"auto", "always", "never"},
			cobra.ShellCompDirectiveNoFileComp,
		),
	))
	must(root.RegisterFlagCompletionFunc(
		"colors",
		cobra.FixedCompletions(
			[]cobra.Completion{"auto", "always", "never"},
			cobra.ShellCompDirectiveNoFileComp,
		),
	))

	root.AddCommand(
		newFormatCommand(state),
		newLintCommand(state),
		newAnalyzeCommand(state),
		newVersionCommand(state),
	)
	return root
}

func newFormatCommand(state *commandState) *cobra.Command {
	options := formatOptions{}
	command := &cobra.Command{
		Use:     "fmt [FILE|DIR]...",
		Aliases: []string{"format"},
		Short:   "Format Go source",
		Args:    cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if command.Flags().Changed("stdin-filename") && !options.stdin {
				return fmt.Errorf("--stdin-filename requires --stdin")
			}
			if options.stdin && len(args) != 0 {
				return fmt.Errorf("--stdin does not accept file or directory arguments")
			}
			options.paths = append([]string(nil), args...)
			if len(options.paths) == 0 && !options.stdin {
				options.paths = []string{"."}
			}
			if !options.stdin && !options.check && !options.diff && !options.write {
				options.write = true
			}

			configuration, colorMode, err := loadConfiguration(state)
			if err != nil {
				return err
			}
			state.code = runFormat(options, configuration, colorMode, state.stdin, state.stdout, state.stderr)
			return nil
		},
	}
	flags := command.Flags()
	flags.BoolVar(&options.check, "check", false, "report files that would change without writing")
	flags.BoolVar(&options.diff, "diff", false, "print full unified diffs without writing")
	flags.BoolVar(&options.write, "write", false, "write formatted source in place")
	flags.BoolVar(&options.stdin, "stdin", false, "read source from stdin and write it to stdout")
	flags.StringVar(&options.stdinFilename, "stdin-filename", "<stdin>", "logical filename for stdin")
	command.MarkFlagsMutuallyExclusive("check", "diff", "write", "stdin")
	must(command.MarkFlagFilename("stdin-filename", "go"))
	return command
}

func newLintCommand(state *commandState) *cobra.Command {
	options := lintOptions{format: "text"}
	command := &cobra.Command{
		Use:   "lint [FILE|DIR]...",
		Short: "Run clarity and safety rules",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			options.paths = append([]string(nil), args...)
			configuration, colorMode, err := loadConfiguration(state)
			if err != nil {
				return err
			}
			state.code = runLint(
				options,
				command.Flags().Changed("baseline"),
				command.Flags().Changed("baseline-variant"),
				configuration,
				colorMode,
				state.stdout,
				state.stderr,
			)
			return nil
		},
	}
	addLintFlags(command, &options)
	addRuleCompletions(command, lintRuleCompletions)
	return command
}

func newAnalyzeCommand(state *commandState) *cobra.Command {
	options := analyzeOptions{format: "text"}
	command := &cobra.Command{
		Use:   "analyze [FILE|DIR]...",
		Short: "Run package-aware static analysis",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			options.paths = append([]string(nil), args...)
			configuration, colorMode, err := loadConfiguration(state)
			if err != nil {
				return err
			}
			state.code = runAnalyze(
				options,
				command.Flags().Changed("baseline"),
				command.Flags().Changed("baseline-variant"),
				configuration,
				colorMode,
				state.stdout,
				state.stderr,
			)
			return nil
		},
	}
	addAnalyzeFlags(command, &options)
	addRuleCompletions(command, analyzeRuleCompletions)
	return command
}

func newVersionCommand(state *commandState) *cobra.Command {
	return &cobra.Command{
		Use:               "version",
		Short:             "Print the version",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(_ *cobra.Command, _ []string) {
			colorMode := ui.ColorAuto
			if state.globals.colorSet {
				colorMode = ui.ColorMode(state.globals.color)
			}
			palette := ui.NewPalette(state.stdout, colorMode)
			fmt.Fprintf(state.stdout, "%s %s\n", palette.Bold("strider"), palette.Accent(version))
		},
	}
}

func addLintFlags(command *cobra.Command, options *lintOptions) {
	flags := command.Flags()
	flags.StringVar(&options.format, "format", options.format, "report format: text or json")
	flags.BoolVar(&options.listRules, "list-rules", false, "list enabled lint rules")
	flags.BoolVar(&options.allRules, "all-rules", false, "run every built-in rule")
	flags.StringVar(&options.explain, "explain", "", "explain a lint rule")
	flags.Var(&options.only, "only", "run only these rule codes (repeatable or comma-separated)")
	addBaselineFlags(command, &options.baselinePath, &options.baselineVariant, &options.generate, &options.prune, &options.ignore, &options.backup)
	command.MarkFlagsMutuallyExclusive("all-rules", "only")
	registerFixedCompletion(command, "format", "text", "json")
}

func addAnalyzeFlags(command *cobra.Command, options *analyzeOptions) {
	flags := command.Flags()
	flags.StringVar(&options.format, "format", options.format, "report format: text or json")
	flags.BoolVar(&options.listRules, "list-rules", false, "list enabled analysis rules")
	flags.StringVar(&options.explain, "explain", "", "explain an analysis rule")
	flags.Var(&options.only, "only", "run only these rule codes (repeatable or comma-separated)")
	addBaselineFlags(command, &options.baselinePath, &options.baselineVariant, &options.generate, &options.prune, &options.ignore, &options.backup)
	registerFixedCompletion(command, "format", "text", "json")
}

func addBaselineFlags(
	command *cobra.Command,
	path, variant *string,
	generate, prune, ignore, backup *bool,
) {
	flags := command.Flags()
	flags.StringVar(path, "baseline", "", "path to the baseline")
	flags.StringVar(variant, "baseline-variant", "", "generated baseline variant: loose or strict")
	flags.BoolVar(generate, "generate-baseline", false, "replace the baseline with all current findings")
	flags.BoolVar(prune, "remove-outdated-baseline-entries", false, "remove baseline entries that no longer match")
	flags.BoolVar(ignore, "ignore-baseline", false, "report findings without applying a baseline")
	flags.BoolVar(backup, "backup-baseline", false, "back up a baseline before replacing it")
	command.MarkFlagsMutuallyExclusive("generate-baseline", "remove-outdated-baseline-entries")
	command.MarkFlagsMutuallyExclusive("ignore-baseline", "generate-baseline")
	command.MarkFlagsMutuallyExclusive("ignore-baseline", "remove-outdated-baseline-entries")
	must(command.MarkFlagFilename("baseline", "toml"))
	registerFixedCompletion(command, "baseline-variant", "loose", "strict")
}

func addRuleCompletions(command *cobra.Command, available func() []cobra.Completion) {
	complete := func(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		completions := available()
		prefix := ""
		if index := strings.LastIndex(toComplete, ","); index >= 0 {
			prefix = toComplete[:index+1]
		}
		result := make([]cobra.Completion, 0, len(completions))
		for _, completion := range completions {
			value, description, _ := strings.Cut(string(completion), "\t")
			if prefix != "" {
				value = prefix + value
			}
			result = append(result, cobra.CompletionWithDesc(value, description))
		}
		return result, cobra.ShellCompDirectiveNoFileComp
	}
	must(command.RegisterFlagCompletionFunc("only", complete))
	must(command.RegisterFlagCompletionFunc("explain", complete))
}

func lintRuleCompletions() []cobra.Completion {
	registry, err := lint.NewRegistryAll()
	if err != nil {
		panic(err)
	}
	completions := make([]cobra.Completion, 0, len(registry.Rules()))
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		completions = append(completions, cobra.CompletionWithDesc(meta.Code, meta.Summary))
	}
	sort.Slice(completions, func(i, j int) bool { return completions[i] < completions[j] })
	return completions
}

func analyzeRuleCompletions() []cobra.Completion {
	registry, err := analyze.NewRegistry(nil)
	if err != nil {
		panic(err)
	}
	completions := make([]cobra.Completion, 0, len(registry.Rules()))
	for _, rule := range registry.Rules() {
		meta := rule.Meta()
		completions = append(completions, cobra.CompletionWithDesc(meta.Code, meta.Summary))
	}
	sort.Slice(completions, func(i, j int) bool { return completions[i] < completions[j] })
	return completions
}

func registerFixedCompletion(command *cobra.Command, name string, values ...string) {
	completions := make([]cobra.Completion, len(values))
	for index, value := range values {
		completions[index] = cobra.Completion(value)
	}
	must(command.RegisterFlagCompletionFunc(
		name,
		cobra.FixedCompletions(completions, cobra.ShellCompDirectiveNoFileComp),
	))
}

func loadConfiguration(state *commandState) (config.Config, ui.ColorMode, error) {
	configuration, err := config.Load(state.globals.configPath, state.globals.noConfig)
	if err != nil {
		return config.Config{}, ui.ColorAuto, err
	}
	return configuration, configuredColor(configuration, state.globals), nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
