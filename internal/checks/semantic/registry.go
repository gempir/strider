package semantic

import (
	"fmt"
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

var ruleCatalog = []ruleDefinition{
	ssaStaticCallDefinition(invalidRegexpRule{}, FactCallArguments, "regexp"),
	typedDefinition(invalidTemplateRule{}, 0),
	ssaStaticCallDefinition(invalidTimeParseRule{}, FactCallArguments, "time"),
	ssaStaticCallDefinition(unsupportedBinaryWriteRule{}, FactCallArguments, "encoding/binary"),
	typedDefinition(suspiciousSleepRule{}, 0),
	typedDefinition(invalidExecCommandRule{}, 0),
	typedDefinition(dynamicPrintfRule{}, 0),
	ssaStaticCallDefinition(invalidURLRule{}, FactCallArguments, "net/url"),
	typedDefinition(nonCanonicalHeaderRule{}, 0),
	ssaStaticCallDefinition(regexpFindAllZeroRule{}, FactCallArguments, "regexp"),
	ssaStaticCallDefinition(invalidUTF8StringArgumentRule{}, FactCallArguments, "strings"),
	typedDefinition(nilContextRule{}, 0),
	typedDefinition(swappedSeekArgumentsRule{}, 0),
	ssaStaticCallDefinition(nonPointerUnmarshalRule{}, FactCallArguments, "encoding/json", "encoding/xml"),
	ssaDefinition(leakyTimeTickRule{}, 0, 0),
	typedDefinition(untrappableSignalRule{}, 0),
	ssaStaticCallDefinition(unbufferedSignalChannelRule{}, FactCallArguments, "os/signal"),
	ssaStaticCallDefinition(zeroReplacementLimitRule{}, FactCallArguments, "bytes", "strings"),
	typedDefinition(deprecatedAPIUsageRule{}, FactDeprecations),
	ssaStaticCallDefinition(invalidListenAddressRule{}, FactCallArguments, "net/http"),
	ssaStaticCallDefinition(ipByteComparisonRule{}, FactCallArguments, "bytes"),
	ssaDefinition(writerBufferMutationRule{}, 0, 0),
	ssaStaticCallDefinition(duplicateTrimCutsetRule{}, FactCallArguments, "strings"),
	ssaDefinition(timerResetDrainRaceRule{}, 0, 0),
	ssaStaticCallDefinition(unsupportedMarshalTypeRule{}, FactCallArguments, "encoding/json", "encoding/xml"),
	ssaStaticCallDefinition(misalignedAtomic64Rule{}, FactCallArguments, "sync/atomic"),
	ssaStaticCallDefinition(sortNonSliceRule{}, FactCallArguments, "sort"),
	ssaStaticCallDefinition(contextKeyTypeRule{}, FactCallArguments, "context"),
	ssaStaticCallDefinition(invalidStrconvArgumentRule{}, FactCallArguments, "strconv"),
	ssaStaticCallDefinition(overlappingEncodeBuffersRule{}, FactCallArguments, "encoding/ascii85", "encoding/base32", "encoding/base64", "encoding/hex"),
	ssaStaticCallDefinition(swappedErrorsIsArgumentsRule{}, FactCallArguments, "errors"),
	typedDefinition(waitGroupAddInsideGoroutineRule{}, 0),
	typedDefinition(emptyCriticalSectionRule{}, 0),
	ssaDefinition(testingFatalInGoroutineRule{}, 0, 0),
	typedDefinition(deferredLockAfterLockRule{}, 0),
	typedDefinition(testMainMissingExitRule{}, 0),
	typedDefinition(timeValueEqualityRule{}, 0),
	typedDefinition(waitGroupGoForbiddenCallRule{}, 0),
	typedDefinition(rangeValueCaptureRule{}, FactParents),
	typedDefinition(benchmarkIterationMutationRule{}, 0),
	typedDefinition(identicalBinaryOperandsRule{}, FactParents),
	typedDefinition(impossibleIntegerComparisonRule{}, 0),
	typedDefinition(singleIterationLoopRule{}, 0),
	ssaDefinition(ineffectiveValueReceiverAssignmentRule{}, 0, 0),
	ssaDefinition(overwrittenBeforeUseRule{}, 0, SSAFeatureGlobalDebug),
	ssaDefinition(unchangedLoopConditionRule{}, 0, SSAFeatureGlobalDebug),
	ssaDefinition(argumentOverwrittenBeforeUseRule{}, 0, 0),
	ssaDefinition(unusedAppendResultRule{}, 0, 0),
	ssaDefinition(nanComparisonRule{}, 0, 0),
	ssaDefinition(pointlessIntegerMathRule{}, 0, 0),
	typedDefinition(ineffectiveBitwiseZeroRule{}, 0),
	ssaDefinition(discardedPureResultRule{}, 0, 0),
	ssaDefinition(selfAssignmentRule{}, 0, 0),
	typedDefinition(unreachableTypeSwitchCaseRule{}, 0),
	typedDefinition(singleArgumentAppendRule{}, 0),
	typedDefinition(addressNilComparisonRule{}, 0),
	ssaDefinition(impossibleInterfaceNilComparisonRule{}, 0, 0),
	typedDefinition(negativeLengthCapacityComparisonRule{}, 0),
	typedDefinition(constantNegativeZeroRule{}, 0),
	typedDefinition(urlQueryCopyMutationRule{}, 0),
	typedDefinition(sortConversionWithoutSortRule{}, 0),
	ssaStaticCallDefinition(randomBoundOneRule{}, 0, "math/rand", "math/rand/v2"),
	ssaDefinition(neverNilComparisonRule{}, 0, SSAFeatureGlobalDebug),
	typedDefinition(impossiblePlatformComparisonRule{}, 0),
	ssaDefinition(nilMapAssignmentRule{}, 0, 0),
	typedDefinition(deferCloseBeforeErrorCheckRule{}, 0),
	typedDefinition(spinningEmptyLoopRule{}, 0),
	ssaStaticCallDefinition(finalizerCapturesObjectRule{}, 0, "runtime"),
	ssaDefinition(infiniteRecursionRule{}, 0, 0),
	typedDefinition(invalidPrintfCallRule{}, 0),
	typedDefinition(contradictoryInterfaceAssertionRule{}, 0),
	ssaDefinition(possibleNilDereferenceRule{}, 0, 0),
	typedDefinition(oddPairedArgumentsRule{}, 0),
	ssaDefinition(regexpMatchInLoopRule{}, 0, 0),
	typedDefinition(separateByteStringMapKeyRule{}, FactParents),
	typedDefinition(nonPointerSyncPoolValueRule{}, 0),
	typedDefinition(caseInsensitiveStringComparisonRule{}, 0),
	typedDefinition(byteStringWriteRule{}, 0),
	typedDefinition(decimalFileModeRule{}, 0),
	typedDefinition(partiallyTypedConstantGroupRule{}, 0),
	ssaStaticCallDefinition(unexportedSerializationFieldsRule{}, FactCallArguments, "encoding/json", "encoding/xml"),
	typedDefinition(oversizedFixedWidthShiftRule{}, 0),
	ssaStaticCallDefinition(dangerousDirectoryRemovalRule{}, 0, "os"),
	typedDefinition(failedAssertionShadowReadRule{}, 0),
	typedDefinition(deferredReturnFunctionNotCalledRule{}, 0),
	typedDefinition(durationMultipliedByDurationRule{}, 0),
	typedDefinition(contextStoredInStructRule{}, 0),
	typedDefinition(unsafeFormattedURLHostPortRule{}, 0),
	typedDefinition(uncheckedRowsErrorRule{}, 0),
	typedDefinition(excessiveBlankIdentifiersRule{}, 0),
	typedDefinition(taskCommentRule{}, 0),
	typedDefinition(docCommentPeriodRule{}, 0),
	typedDefinition(errorTypeNamingRule{}, 0),
	typedDefinition(standardHTTPMethodConstantRule{}, 0),
	typedDefinition(weakCryptographyRule{}, 0),
	ssaDefinition(appendToSizedSliceRule{}, 0, 0),
	typedDefinition(redundantConversionRule{}, 0),
	typedDefinition(slicePreallocationRule{}, 0),
	typedDefinition(inefficientSprintfRule{}, 0),
	typedDefinition(interfaceMethodLimitRule{}, 0),
	typedDefinition(constructorInterfaceReturnRule{}, 0),
	typedDefinition(slogArgumentShapeRule{}, 0),
	ssaDefinition(externalCallInLoopRule{}, 0, 0),
	typedDefinition(nilErrorReturnRule{}, 0),
	typedDefinition(nilValueWithNilErrorRule{}, 0),
	typedDefinition(unclosedHTTPResponseBodyRule{}, 0),
	typedDefinition(unclosedSQLResourceRule{}, 0),
	typedDefinition(contextCancelInLoopRule{}, 0),
	typedDefinition(copyLockValueRule{}, 0),
	typedDefinition(discardedErrorResultRule{}, 0),
	typedDefinition(testParallelismRule{}, 0),
	typedDefinition(topLevelDeclarationOrderRule{}, 0),
}

// Registry is an immutable selection of analysis rules.
type Registry struct {
	rules       []Rule
	definitions []ruleDefinition
	settings    map[string]configuredRule
	knownCodes  map[string]bool
	root        string
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

type ruleDefinition struct {
	rule         Rule
	requirements Requirements
}

type executionPlan struct {
	requirements       Requirements
	staticCallPackages map[string]bool
}

// Has reports whether all wanted facts are included in the set.
func (facts FactSet) Has(wanted FactSet) bool {
	return facts&wanted == wanted
}

func compileExecutionPlan(definitions []ruleDefinition) executionPlan {
	plan := executionPlan{}
	for _, definition := range definitions {
		requirements := definition.requirements
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
	return compileExecutionPlan(registry.definitions)
}

// NewRegistry applies project settings and a minimum effective severity.
// Explicit selection never bypasses the severity threshold. The []string
// input remains as a narrow compatibility shim for callers that only select
// checks; new callers use RegistryOptions.
func NewRegistry(input any) (*Registry, error) {
	var options RegistryOptions
	switch value := input.(type) {
	case nil:
	case []string:
		options.Only = value
	case RegistryOptions:
		options = value
	default:
		return nil, fmt.Errorf("semantic registry options must be RegistryOptions or []string, got %T", input)
	}
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
		registry.definitions = append(registry.definitions, definitionFor(rule))
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
	return pathfilter.Matches(registry.root, filename, registry.settings[code].excludes)
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
	for _, definition := range ruleCatalog {
		if strings.EqualFold(definition.rule.Meta().Code, code) {
			return definition.requirements, true
		}
	}
	return Requirements{}, false
}

func definitionFor(rule Rule) ruleDefinition {
	for _, definition := range ruleCatalog {
		if strings.EqualFold(definition.rule.Meta().Code, rule.Meta().Code) {
			return definition
		}
	}
	return ruleDefinition{
		rule: rule,
		requirements: Requirements{
			Stage: AnalysisStageTypes,
		},
	}
}

func typedDefinition(rule Rule, facts FactSet) ruleDefinition {
	return ruleDefinition{
		rule: rule,
		requirements: Requirements{
			Stage: AnalysisStageTypes,
			Facts: facts,
		},
	}
}

func ssaDefinition(rule Rule, facts FactSet, features SSAFeatureSet) ruleDefinition {
	return ruleDefinition{
		rule: rule,
		requirements: Requirements{
			Stage:       AnalysisStageSSA,
			Facts:       facts,
			SSAFeatures: features,
		},
	}
}

func ssaStaticCallDefinition(rule Rule, facts FactSet, packagePaths ...string) ruleDefinition {
	definition := ssaDefinition(rule, facts|FactStaticCalls, 0)
	definition.requirements.staticCallPackages = append([]string(nil), packagePaths...)
	return definition
}

func allRules() []Rule {
	rules := make([]Rule, 0, len(ruleCatalog))
	for _, definition := range ruleCatalog {
		rules = append(rules, definition.rule)
	}
	return rules
}
