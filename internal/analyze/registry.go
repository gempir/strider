package analyze

import (
	"fmt"
	"sort"
	"strings"
)

// Registry is an immutable selection of analysis rules.
type Registry struct {
	rules []Rule
}

// NewRegistry selects all implemented rules, or only the explicitly named
// rules when only is non-empty. Rule codes are case-insensitive.
func NewRegistry(only []string) (*Registry, error) {
	all := allRules()
	byCode := make(map[string]Rule, len(all))
	for _, rule := range all {
		byCode[strings.ToUpper(rule.Meta().Code)] = rule
	}

	wanted := make(map[string]bool, len(only))
	original := make(map[string]string, len(only))
	for _, code := range only {
		normalized := strings.ToUpper(code)
		wanted[normalized] = true
		original[normalized] = code
	}
	unknown := make([]string, 0)
	for code := range wanted {
		if byCode[code] == nil {
			unknown = append(unknown, original[code])
		}
	}
	if len(unknown) != 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown analysis rule(s): %s", strings.Join(unknown, ", "))
	}

	selected := make([]Rule, 0, len(all))
	for _, rule := range all {
		if len(wanted) != 0 && !wanted[strings.ToUpper(rule.Meta().Code)] {
			continue
		}
		selected = append(selected, rule)
	}
	return &Registry{rules: selected}, nil
}

// Rules returns a copy of the selected rules.
func (registry *Registry) Rules() []Rule {
	return append([]Rule(nil), registry.rules...)
}

func (registry *Registry) hasRule(code string) bool {
	for _, rule := range registry.rules {
		if rule.Meta().Code == code {
			return true
		}
	}
	return false
}

func allRules() []Rule {
	return []Rule{
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
	}
}
