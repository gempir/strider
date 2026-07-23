//strider:ignore-file cognitive-complexity,no-package-var,top-level-declaration-order
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

var checkCatalog = []Descriptor{
	semanticCheck(invalidRegexpCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"regexp",
		},
	}),
	typeCheck(invalidTemplateCheck{}),
	semanticCheck(invalidTimeParseCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"time",
		},
	}),
	semanticCheck(
		unsupportedBinaryWriteCheck{},
		Requirements{
			Stage: AnalysisStageSSA,
			Facts: FactCallArguments | FactStaticCalls,
			staticCallPackages: []string{
				"encoding/binary",
			},
		},
	),
	typeCheck(suspiciousSleepCheck{}),
	typeCheck(invalidExecCommandCheck{}),
	typeCheck(dynamicPrintfCheck{}),
	semanticCheck(invalidURLCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"net/url",
		},
	}),
	typeCheck(nonCanonicalHeaderCheck{}),
	semanticCheck(regexpFindAllZeroCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"regexp",
		},
	}),
	semanticCheck(invalidUTF8StringArgumentCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"strings",
		},
	}),
	typeCheck(nilContextCheck{}),
	typeCheck(swappedSeekArgumentsCheck{}),
	semanticCheck(
		nonPointerUnmarshalCheck{},
		Requirements{
			Stage: AnalysisStageSSA,
			Facts: FactCallArguments | FactStaticCalls,
			staticCallPackages: []string{
				"encoding/json",
				"encoding/xml",
			},
		},
	),
	ssaCheck(leakyTimeTickCheck{}),
	typeCheck(untrappableSignalCheck{}),
	semanticCheck(unbufferedSignalChannelCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"os/signal",
		},
	}),
	semanticCheck(
		zeroReplacementLimitCheck{},
		Requirements{
			Stage: AnalysisStageSSA,
			Facts: FactCallArguments | FactStaticCalls,
			staticCallPackages: []string{
				"bytes",
				"strings",
			},
		},
	),
	semanticCheck(deprecatedAPIUsageCheck{}, Requirements{
		Stage: AnalysisStageTypes,
		Facts: FactDeprecations,
	}),
	semanticCheck(invalidListenAddressCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"net/http",
		},
	}),
	semanticCheck(ipByteComparisonCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"bytes",
		},
	}),
	ssaCheck(writerBufferMutationCheck{}),
	semanticCheck(duplicateTrimCutsetCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"strings",
		},
	}),
	ssaCheck(timerResetDrainRaceCheck{}),
	semanticCheck(
		unsupportedMarshalTypeCheck{},
		Requirements{
			Stage: AnalysisStageSSA,
			Facts: FactCallArguments | FactStaticCalls,
			staticCallPackages: []string{
				"encoding/json",
				"encoding/xml",
			},
		},
	),
	semanticCheck(misalignedAtomic64Check{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"sync/atomic",
		},
	}),
	semanticCheck(sortNonSliceCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"sort",
		},
	}),
	semanticCheck(contextKeyTypeCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"context",
		},
	}),
	semanticCheck(invalidStrconvArgumentCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"strconv",
		},
	}),
	semanticCheck(
		overlappingEncodeBuffersCheck{},
		Requirements{
			Stage: AnalysisStageSSA,
			Facts: FactCallArguments | FactStaticCalls,
			staticCallPackages: []string{
				"encoding/ascii85",
				"encoding/base32",
				"encoding/base64",
				"encoding/hex",
			},
		},
	),
	semanticCheck(swappedErrorsIsArgumentsCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactCallArguments | FactStaticCalls,
		staticCallPackages: []string{
			"errors",
		},
	}),
	typeCheck(waitGroupAddInsideGoroutineCheck{}),
	typeCheck(emptyCriticalSectionCheck{}),
	ssaCheck(testingFatalInGoroutineCheck{}),
	typeCheck(deferredLockAfterLockCheck{}),
	typeCheck(testMainMissingExitCheck{}),
	typeCheck(timeValueEqualityCheck{}),
	typeCheck(waitGroupGoForbiddenCallCheck{}),
	semanticCheck(rangeValueCaptureCheck{}, Requirements{
		Stage: AnalysisStageTypes,
		Facts: FactParents,
	}),
	typeCheck(benchmarkIterationMutationCheck{}),
	semanticCheck(identicalBinaryOperandsCheck{}, Requirements{
		Stage: AnalysisStageTypes,
		Facts: FactParents,
	}),
	typeCheck(impossibleIntegerComparisonCheck{}),
	typeCheck(singleIterationLoopCheck{}),
	ssaCheck(ineffectiveValueReceiverAssignmentCheck{}),
	semanticCheck(overwrittenBeforeUseCheck{}, Requirements{
		Stage:       AnalysisStageSSA,
		SSAFeatures: SSAFeatureGlobalDebug,
	}),
	semanticCheck(unchangedLoopConditionCheck{}, Requirements{
		Stage:       AnalysisStageSSA,
		SSAFeatures: SSAFeatureGlobalDebug,
	}),
	ssaCheck(argumentOverwrittenBeforeUseCheck{}),
	ssaCheck(unusedAppendResultCheck{}),
	ssaCheck(nanComparisonCheck{}),
	ssaCheck(pointlessIntegerMathCheck{}),
	typeCheck(ineffectiveBitwiseZeroCheck{}),
	ssaCheck(discardedPureResultCheck{}),
	ssaCheck(selfAssignmentCheck{}),
	typeCheck(unreachableTypeSwitchCaseCheck{}),
	typeCheck(singleArgumentAppendCheck{}),
	typeCheck(addressNilComparisonCheck{}),
	ssaCheck(impossibleInterfaceNilComparisonCheck{}),
	typeCheck(negativeLengthCapacityComparisonCheck{}),
	typeCheck(constantNegativeZeroCheck{}),
	typeCheck(urlQueryCopyMutationCheck{}),
	typeCheck(sortConversionWithoutSortCheck{}),
	semanticCheck(randomBoundOneCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactStaticCalls,
		staticCallPackages: []string{
			"math/rand",
			"math/rand/v2",
		},
	}),
	semanticCheck(neverNilComparisonCheck{}, Requirements{
		Stage:       AnalysisStageSSA,
		SSAFeatures: SSAFeatureGlobalDebug,
	}),
	typeCheck(impossiblePlatformComparisonCheck{}),
	ssaCheck(nilMapAssignmentCheck{}),
	typeCheck(deferCloseBeforeErrorCheckCheck{}),
	typeCheck(spinningEmptyLoopCheck{}),
	semanticCheck(finalizerCapturesObjectCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactStaticCalls,
		staticCallPackages: []string{
			"runtime",
		},
	}),
	ssaCheck(infiniteRecursionCheck{}),
	typeCheck(contradictoryInterfaceAssertionCheck{}),
	typeCheck(oddPairedArgumentsCheck{}),
	ssaCheck(regexpMatchInLoopCheck{}),
	semanticCheck(separateByteStringMapKeyCheck{}, Requirements{
		Stage: AnalysisStageTypes,
		Facts: FactParents,
	}),
	typeCheck(nonPointerSyncPoolValueCheck{}),
	typeCheck(caseInsensitiveStringComparisonCheck{}),
	typeCheck(byteStringWriteCheck{}),
	typeCheck(decimalFileModeCheck{}),
	typeCheck(partiallyTypedConstantGroupCheck{}),
	semanticCheck(
		unexportedSerializationFieldsCheck{},
		Requirements{
			Stage: AnalysisStageSSA,
			Facts: FactCallArguments | FactStaticCalls,
			staticCallPackages: []string{
				"encoding/json",
				"encoding/xml",
			},
		},
	),
	typeCheck(oversizedFixedWidthShiftCheck{}),
	semanticCheck(dangerousDirectoryRemovalCheck{}, Requirements{
		Stage: AnalysisStageSSA,
		Facts: FactStaticCalls,
		staticCallPackages: []string{
			"os",
		},
	}),
	typeCheck(deferredReturnFunctionNotCalledCheck{}),
	typeCheck(durationMultipliedByDurationCheck{}),
	typeCheck(contextStoredInStructCheck{}),
	typeCheck(unsafeFormattedURLHostPortCheck{}),
	typeCheck(uncheckedRowsErrorCheck{}),
	typeCheck(errorTypeNamingCheck{}),
	typeCheck(standardHTTPMethodConstantCheck{}),
	typeCheck(weakCryptographyCheck{}),
	ssaCheck(appendToSizedSliceCheck{}),
	typeCheck(redundantConversionCheck{}),
	typeCheck(slicePreallocationCheck{}),
	typeCheck(inefficientSprintfCheck{}),
	typeCheck(interfaceMethodLimitCheck{}),
	typeCheck(constructorInterfaceReturnCheck{}),
	typeCheck(slogArgumentShapeCheck{}),
	typeCheck(nilErrorReturnCheck{}),
	typeCheck(nilValueWithNilErrorCheck{}),
	typeCheck(unclosedHTTPResponseBodyCheck{}),
	typeCheck(unclosedSQLResourceCheck{}),
	typeCheck(contextCancelInLoopCheck{}),
	typeCheck(copyLockValueCheck{}),
	typeCheck(discardedErrorResultCheck{}),
}

// Plan is an immutable selection of analysis checks.
type Plan struct {
	checks    []Descriptor
	settings  map[string]configuredCheck
	root      string
	rootSet   bool
	directory string
}

type configuredCheck struct {
	severity diagnostic.Severity
	excludes []string
	options  catalog.ResolvedOptions
}

// SelectedCheck is a fully bound semantic check produced by the unified
// selection boundary.
type SelectedCheck struct {
	Check    Descriptor
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

// Descriptor binds a semantic implementation to its explicit analysis
// requirements. Implementations only provide metadata and Run; registration
// chooses the type or SSA stage so expensive needs cannot be inferred
// accidentally.
type Descriptor struct {
	check        Check
	requirements Requirements
}

func typeCheck(check Check) Descriptor {
	return semanticCheck(check, Requirements{
		Stage: AnalysisStageTypes,
	})
}

func ssaCheck(check Check) Descriptor {
	return semanticCheck(check, Requirements{
		Stage: AnalysisStageSSA,
	})
}

func semanticCheck(check Check, requirements Requirements) Descriptor {
	return Descriptor{
		check:        check,
		requirements: requirements,
	}
}

func (descriptor Descriptor) Meta() Meta {
	return descriptor.check.Meta()
}

func (descriptor Descriptor) Run(pass *Pass) {
	descriptor.check.Run(pass)
}

type executionPlan struct {
	requirements       Requirements
	staticCallPackages map[string]bool
}

// Has reports whether all wanted facts are included in the set.
func (facts FactSet) Has(wanted FactSet) bool {
	return facts&wanted == wanted
}

func compileExecutionPlan(checks []Descriptor) executionPlan {
	plan := executionPlan{}
	for _, check := range checks {
		requirements := check.requirements
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
func Catalog() []Descriptor {
	return append([]Descriptor(nil), checkCatalog...)
}

// NewPlan prepares semantic execution from already-selected, schema-bound
// checks. It deliberately has no selection or configuration policy.
func NewPlan(selected []SelectedCheck, root string, rootSet bool, directory ...string) *Plan {
	registry := &Plan{
		settings:  make(map[string]configuredCheck, len(selected)),
		root:      source.ResolveRoot(root),
		rootSet:   rootSet,
		directory: ".",
	}
	if len(directory) != 0 && directory[0] != "" {
		registry.directory = source.ResolveRoot(directory[0])
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
