package semantic

import (
	"strings"

	"github.com/gempir/strider/internal/checks/core"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/pathfilter"
)

const (
	AnalysisStageTypes AnalysisStage = iota + 1
	AnalysisStageSSA
)

const (
	FactCallArguments FactSet = 1 << iota
	FactParents
	FactDeprecations
	// FactStaticCalls indexes statically resolved SSA calls by callee package.
	// Rules using this fact avoid walking every instruction independently.
	FactStaticCalls
)

const (
	SSAFeatureGlobalDebug SSAFeatureSet = 1 << iota
)

var ruleCatalog = []Rule{
	invalidRegexpRule{},
	invalidTemplateRule{},
	invalidTimeParseRule{},
	unsupportedBinaryWriteRule{},
	suspiciousSleepRule{},
	invalidExecCommandRule{},
	dynamicPrintfRule{},
	invalidURLRule{},
	nonCanonicalHeaderRule{},
	regexpFindAllZeroRule{},
	invalidUTF8StringArgumentRule{},
	nilContextRule{},
	swappedSeekArgumentsRule{},
	nonPointerUnmarshalRule{},
	leakyTimeTickRule{},
	untrappableSignalRule{},
	unbufferedSignalChannelRule{},
	zeroReplacementLimitRule{},
	deprecatedAPIUsageRule{},
	invalidListenAddressRule{},
	ipByteComparisonRule{},
	writerBufferMutationRule{},
	duplicateTrimCutsetRule{},
	timerResetDrainRaceRule{},
	unsupportedMarshalTypeRule{},
	misalignedAtomic64Rule{},
	sortNonSliceRule{},
	contextKeyTypeRule{},
	invalidStrconvArgumentRule{},
	overlappingEncodeBuffersRule{},
	swappedErrorsIsArgumentsRule{},
	waitGroupAddInsideGoroutineRule{},
	emptyCriticalSectionRule{},
	testingFatalInGoroutineRule{},
	deferredLockAfterLockRule{},
	testMainMissingExitRule{},
	timeValueEqualityRule{},
	waitGroupGoForbiddenCallRule{},
	rangeValueCaptureRule{},
	benchmarkIterationMutationRule{},
	identicalBinaryOperandsRule{},
	impossibleIntegerComparisonRule{},
	singleIterationLoopRule{},
	ineffectiveValueReceiverAssignmentRule{},
	overwrittenBeforeUseRule{},
	unchangedLoopConditionRule{},
	argumentOverwrittenBeforeUseRule{},
	unusedAppendResultRule{},
	nanComparisonRule{},
	pointlessIntegerMathRule{},
	ineffectiveBitwiseZeroRule{},
	discardedPureResultRule{},
	selfAssignmentRule{},
	unreachableTypeSwitchCaseRule{},
	singleArgumentAppendRule{},
	addressNilComparisonRule{},
	impossibleInterfaceNilComparisonRule{},
	negativeLengthCapacityComparisonRule{},
	constantNegativeZeroRule{},
	urlQueryCopyMutationRule{},
	sortConversionWithoutSortRule{},
	randomBoundOneRule{},
	neverNilComparisonRule{},
	impossiblePlatformComparisonRule{},
	nilMapAssignmentRule{},
	deferCloseBeforeErrorCheckRule{},
	spinningEmptyLoopRule{},
	finalizerCapturesObjectRule{},
	infiniteRecursionRule{},
	invalidPrintfCallRule{},
	contradictoryInterfaceAssertionRule{},
	possibleNilDereferenceRule{},
	oddPairedArgumentsRule{},
	regexpMatchInLoopRule{},
	separateByteStringMapKeyRule{},
	nonPointerSyncPoolValueRule{},
	caseInsensitiveStringComparisonRule{},
	byteStringWriteRule{},
	decimalFileModeRule{},
	partiallyTypedConstantGroupRule{},
	unexportedSerializationFieldsRule{},
	oversizedFixedWidthShiftRule{},
	dangerousDirectoryRemovalRule{},
	failedAssertionShadowReadRule{},
	deferredReturnFunctionNotCalledRule{},
	durationMultipliedByDurationRule{},
	contextStoredInStructRule{},
	unsafeFormattedURLHostPortRule{},
	uncheckedRowsErrorRule{},
	excessiveBlankIdentifiersRule{},
	taskCommentRule{},
	docCommentPeriodRule{},
	errorTypeNamingRule{},
	standardHTTPMethodConstantRule{},
	weakCryptographyRule{},
	appendToSizedSliceRule{},
	redundantConversionRule{},
	slicePreallocationRule{},
	inefficientSprintfRule{},
	interfaceMethodLimitRule{},
	constructorInterfaceReturnRule{},
	slogArgumentShapeRule{},
	externalCallInLoopRule{},
	nilErrorReturnRule{},
	nilValueWithNilErrorRule{},
	unclosedHTTPResponseBodyRule{},
	unclosedSQLResourceRule{},
	contextCancelInLoopRule{},
	copyLockValueRule{},
	discardedErrorResultRule{},
	testParallelismRule{},
	topLevelDeclarationOrderRule{},
}

// Registry is an immutable selection of analysis rules.
type Registry struct {
	rules      []Rule
	settings   map[string]configuredRule
	knownCodes map[string]bool
	root       string
}

type configuredRule struct {
	severity diagnostic.Severity
	excludes []string
	config   config.RuleConfig
}

// RegistryOptions selects and configures package-aware rules.
type RegistryOptions struct {
	Only            []string
	Settings        map[string]config.RuleConfig
	Root            string
	MinimumSeverity diagnostic.Severity
}

// AnalysisStage is the most expensive program representation needed by a
// rule. Every current syntax rule is type-aware, so there is deliberately no
// untyped AST stage yet.
type AnalysisStage uint8

// FactSet identifies shared, lazily constructed analysis indexes. Facts are
// orthogonal to the representation stage: typed and SSA rules can both depend
// on them.
type FactSet uint8

// SSAFeatureSet identifies optional SSA metadata that is expensive enough to
// build only when a selected rule consumes it.
type SSAFeatureSet uint8

// Requirements describes the internal data dependencies of one rule.
type Requirements struct {
	Stage              AnalysisStage
	Facts              FactSet
	SSAFeatures        SSAFeatureSet
	staticCallPackages []string
}

type executionPlan struct {
	requirements       Requirements
	staticCallPackages map[string]bool
}

// Has reports whether all wanted facts are included in the set.
func (facts FactSet) Has(wanted FactSet) bool {
	return facts&wanted == wanted
}

func compileExecutionPlan(rules []Rule) executionPlan {
	plan := executionPlan{}
	for _, rule := range rules {
		requirements := rule.Requirements()
		if requirements.Stage > plan.requirements.Stage {
			plan.requirements.Stage = requirements.Stage
		}
		plan.requirements.Facts |= requirements.Facts
		plan.requirements.SSAFeatures |= requirements.SSAFeatures
		for _, packagePath := range requirements.staticCallPackages {
			if plan.staticCallPackages == nil {
				plan.staticCallPackages = make(map[string]bool)
			}
			plan.staticCallPackages[packagePath] = true
		}
	}
	return plan
}

func (plan executionPlan) needsSSA() bool {
	return plan.requirements.Stage == AnalysisStageSSA
}

func (registry *Registry) executionPlan() executionPlan {
	if registry == nil {
		return executionPlan{}
	}
	return compileExecutionPlan(registry.rules)
}

// NewRegistry applies project settings and a minimum effective severity.
// Explicit selection never bypasses the severity threshold.
func NewRegistry(options RegistryOptions) (*Registry, error) {
	all := allRules()
	selection, err := core.Select(core.SelectionOptions[Rule]{
		Checks:          all,
		Only:            options.Only,
		Settings:        options.Settings,
		MinimumSeverity: options.MinimumSeverity,
	})
	if err != nil {
		return nil, err
	}
	registry := &Registry{
		settings:   make(map[string]configuredRule, len(all)),
		knownCodes: selection.KnownCodes,
		root:       options.Root,
	}
	for _, rule := range selection.Checks {
		meta := rule.Meta()
		setting := selection.Settings[strings.ToLower(meta.Code)]
		severity := selection.Severities[meta.Code]
		registry.rules = append(registry.rules, rule)
		registry.settings[meta.Code] = configuredRule{
			severity: severity,
			excludes: setting.Excludes,
			config:   setting,
		}
	}
	return registry, nil
}

func (registry *Registry) Severity(code string) diagnostic.Severity {
	return registry.settings[code].severity
}

func (registry *Registry) Excluded(code, filename string) bool {
	return pathfilter.Excluded(registry.root, filename, registry.settings[code].excludes)
}

// Rules returns a copy of the selected rules.
func (registry *Registry) Rules() []Rule {
	return append([]Rule(nil), registry.rules...)
}

// KnownCodes returns every package-aware rule code, including rules that are
// disabled or below the current severity threshold.
func (registry *Registry) KnownCodes() map[string]bool {
	if registry == nil {
		return nil
	}
	result := make(map[string]bool, len(registry.knownCodes))
	for code := range registry.knownCodes {
		result[code] = true
	}
	return result
}

// UsesSSA reports whether code requires the SSA capability.
func UsesSSA(code string) bool {
	requirements, ok := RequirementsFor(code)
	return ok && requirements.Stage == AnalysisStageSSA
}

// RequirementsFor returns the colocated requirements for code.
func RequirementsFor(code string) (Requirements, bool) {
	for _, rule := range ruleCatalog {
		if strings.EqualFold(rule.Meta().Code, code) {
			return rule.Requirements(), true
		}
	}
	return Requirements{}, false
}

func allRules() []Rule {
	return append([]Rule(nil), ruleCatalog...)
}
