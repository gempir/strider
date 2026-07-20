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
	// Checks using this fact avoid walking every instruction independently.
	FactStaticCalls
)

const (
	SSAFeatureGlobalDebug SSAFeatureSet = 1 << iota
)

var checkCatalog = []Check{
	invalidRegexpCheck{},
	invalidTemplateCheck{},
	invalidTimeParseCheck{},
	unsupportedBinaryWriteCheck{},
	suspiciousSleepCheck{},
	invalidExecCommandCheck{},
	dynamicPrintfCheck{},
	invalidURLCheck{},
	nonCanonicalHeaderCheck{},
	regexpFindAllZeroCheck{},
	invalidUTF8StringArgumentCheck{},
	nilContextCheck{},
	swappedSeekArgumentsCheck{},
	nonPointerUnmarshalCheck{},
	leakyTimeTickCheck{},
	untrappableSignalCheck{},
	unbufferedSignalChannelCheck{},
	zeroReplacementLimitCheck{},
	deprecatedAPIUsageCheck{},
	invalidListenAddressCheck{},
	ipByteComparisonCheck{},
	writerBufferMutationCheck{},
	duplicateTrimCutsetCheck{},
	timerResetDrainRaceCheck{},
	unsupportedMarshalTypeCheck{},
	misalignedAtomic64Check{},
	sortNonSliceCheck{},
	contextKeyTypeCheck{},
	invalidStrconvArgumentCheck{},
	overlappingEncodeBuffersCheck{},
	swappedErrorsIsArgumentsCheck{},
	waitGroupAddInsideGoroutineCheck{},
	emptyCriticalSectionCheck{},
	testingFatalInGoroutineCheck{},
	deferredLockAfterLockCheck{},
	testMainMissingExitCheck{},
	timeValueEqualityCheck{},
	waitGroupGoForbiddenCallCheck{},
	rangeValueCaptureCheck{},
	benchmarkIterationMutationCheck{},
	identicalBinaryOperandsCheck{},
	impossibleIntegerComparisonCheck{},
	singleIterationLoopCheck{},
	ineffectiveValueReceiverAssignmentCheck{},
	overwrittenBeforeUseCheck{},
	unchangedLoopConditionCheck{},
	argumentOverwrittenBeforeUseCheck{},
	unusedAppendResultCheck{},
	nanComparisonCheck{},
	pointlessIntegerMathCheck{},
	ineffectiveBitwiseZeroCheck{},
	discardedPureResultCheck{},
	selfAssignmentCheck{},
	unreachableTypeSwitchCaseCheck{},
	singleArgumentAppendCheck{},
	addressNilComparisonCheck{},
	impossibleInterfaceNilComparisonCheck{},
	negativeLengthCapacityComparisonCheck{},
	constantNegativeZeroCheck{},
	urlQueryCopyMutationCheck{},
	sortConversionWithoutSortCheck{},
	randomBoundOneCheck{},
	neverNilComparisonCheck{},
	impossiblePlatformComparisonCheck{},
	nilMapAssignmentCheck{},
	deferCloseBeforeErrorCheckCheck{},
	spinningEmptyLoopCheck{},
	finalizerCapturesObjectCheck{},
	infiniteRecursionCheck{},
	invalidPrintfCallCheck{},
	contradictoryInterfaceAssertionCheck{},
	possibleNilDereferenceCheck{},
	oddPairedArgumentsCheck{},
	regexpMatchInLoopCheck{},
	separateByteStringMapKeyCheck{},
	nonPointerSyncPoolValueCheck{},
	caseInsensitiveStringComparisonCheck{},
	byteStringWriteCheck{},
	decimalFileModeCheck{},
	partiallyTypedConstantGroupCheck{},
	unexportedSerializationFieldsCheck{},
	oversizedFixedWidthShiftCheck{},
	dangerousDirectoryRemovalCheck{},
	failedAssertionShadowReadCheck{},
	deferredReturnFunctionNotCalledCheck{},
	durationMultipliedByDurationCheck{},
	contextStoredInStructCheck{},
	unsafeFormattedURLHostPortCheck{},
	uncheckedRowsErrorCheck{},
	excessiveBlankIdentifiersCheck{},
	taskCommentCheck{},
	docCommentPeriodCheck{},
	errorTypeNamingCheck{},
	standardHTTPMethodConstantCheck{},
	weakCryptographyCheck{},
	appendToSizedSliceCheck{},
	redundantConversionCheck{},
	slicePreallocationCheck{},
	inefficientSprintfCheck{},
	interfaceMethodLimitCheck{},
	constructorInterfaceReturnCheck{},
	slogArgumentShapeCheck{},
	externalCallInLoopCheck{},
	nilErrorReturnCheck{},
	nilValueWithNilErrorCheck{},
	unclosedHTTPResponseBodyCheck{},
	unclosedSQLResourceCheck{},
	contextCancelInLoopCheck{},
	copyLockValueCheck{},
	discardedErrorResultCheck{},
	testParallelismCheck{},
	topLevelDeclarationOrderCheck{},
}

// Registry is an immutable selection of analysis checks.
type Registry struct {
	checks     []Check
	settings   map[string]configuredCheck
	knownCodes map[string]bool
	root       string
}

type configuredCheck struct {
	severity diagnostic.Severity
	excludes []string
	config   config.CheckConfig
}

// RegistryOptions selects and configures package-aware checks.
type RegistryOptions struct {
	Only            []string
	Settings        map[string]config.CheckConfig
	Root            string
	MinimumSeverity diagnostic.Severity
}

// AnalysisStage is the most expensive program representation needed by a
// check. Every current syntax check is type-aware, so there is deliberately no
// untyped AST stage yet.
type AnalysisStage uint8

// FactSet identifies shared, lazily constructed analysis indexes. Facts are
// orthogonal to the representation stage: typed and SSA checks can both depend
// on them.
type FactSet uint8

// SSAFeatureSet identifies optional SSA metadata that is expensive enough to
// build only when a selected check consumes it.
type SSAFeatureSet uint8

// Requirements describes the internal data dependencies of one check.
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

func compileExecutionPlan(checks []Check) executionPlan {
	plan := executionPlan{}
	for _, check := range checks {
		requirements := check.Requirements()
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
	return compileExecutionPlan(registry.checks)
}

// NewRegistry applies project settings and a minimum effective severity.
// Explicit selection never bypasses the severity threshold.
func NewRegistry(options RegistryOptions) (*Registry, error) {
	all := allChecks()
	selection, err := core.Select(core.SelectionOptions[Check]{
		Checks:          all,
		Only:            options.Only,
		Settings:        options.Settings,
		MinimumSeverity: options.MinimumSeverity,
	})
	if err != nil {
		return nil, err
	}
	registry := &Registry{
		settings:   make(map[string]configuredCheck, len(all)),
		knownCodes: selection.KnownCodes,
		root:       options.Root,
	}
	for _, check := range selection.Checks {
		meta := check.Meta()
		setting := selection.Settings[strings.ToLower(meta.Code)]
		severity := selection.Severities[meta.Code]
		registry.checks = append(registry.checks, check)
		registry.settings[meta.Code] = configuredCheck{
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

// Checks returns a copy of the selected checks.
func (registry *Registry) Checks() []Check {
	return append([]Check(nil), registry.checks...)
}

// KnownCodes returns every package-aware check code, including checks that are
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
	for _, check := range checkCatalog {
		if strings.EqualFold(check.Meta().Code, code) {
			return check.Requirements(), true
		}
	}
	return Requirements{}, false
}

func allChecks() []Check {
	return append([]Check(nil), checkCatalog...)
}
