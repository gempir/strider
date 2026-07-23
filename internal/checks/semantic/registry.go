package semantic

import (
	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
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
	nilErrorReturnCheck{},
	nilValueWithNilErrorCheck{},
	unclosedHTTPResponseBodyCheck{},
	unclosedSQLResourceCheck{},
	contextCancelInLoopCheck{},
	copyLockValueCheck{},
	discardedErrorResultCheck{},
}

// Plan is an immutable selection of analysis checks.
type Plan struct {
	checks   []Check
	settings map[string]configuredCheck
	root     string
	rootSet  bool
}

type configuredCheck struct {
	severity diagnostic.Severity
	excludes []string
	options  catalog.ResolvedOptions
}

// SelectedCheck is a fully bound semantic check produced by the unified
// selection boundary.
type SelectedCheck struct {
	Check    Check
	Severity diagnostic.Severity
	Excludes []string
	Options  catalog.ResolvedOptions
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

func (registry *Plan) executionPlan() executionPlan {
	if registry == nil {
		return executionPlan{}
	}
	return compileExecutionPlan(registry.checks)
}

// Catalog returns the semantic engine's immutable descriptor catalog.
func Catalog() []Check {
	return append([]Check(nil), checkCatalog...)
}

// NewPlan prepares semantic execution from already-selected, schema-bound
// checks. It deliberately has no selection or configuration policy.
func NewPlan(selected []SelectedCheck, root string, rootSet bool) *Plan {
	registry := &Plan{
		settings: make(map[string]configuredCheck, len(selected)),
		root:     source.ResolveRoot(root),
		rootSet:  rootSet,
	}
	for _, item := range selected {
		meta := item.Check.Meta()
		registry.checks = append(registry.checks, item.Check)
		registry.settings[meta.Code] = configuredCheck{
			severity: item.Severity,
			excludes: append([]string(nil), item.Excludes...),
			options:  item.Options,
		}
	}
	return registry
}

func (registry *Plan) Severity(code string) diagnostic.Severity {
	return registry.settings[code].severity
}

func (registry *Plan) Excluded(code, filename string) bool {
	return pathfilter.Excluded(registry.root, filename, registry.settings[code].excludes)
}
