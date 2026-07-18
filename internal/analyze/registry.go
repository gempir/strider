package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/pathfilter"
)

// Registry is an immutable selection of analysis rules.
type Registry struct {
	rules []Rule
	settings map[string]configuredRule
	root string
}

type configuredRule struct {
	severity diagnostic.Severity
	excludes []string
}

// NewRegistry selects all implemented rules, or only the explicitly named
// rules when only is non-empty. Rule codes are case-insensitive.
func NewRegistry(only []string) (*Registry, error) {
	return NewRegistryConfigured(only, nil, "")
}

// NewRegistryConfigured applies analyzer rule settings. Explicit --only
// selection enables the named analyzers even when configuration disables them.
func NewRegistryConfigured(only []string, settings map[string]config.RuleConfig, root string) (
	*Registry,
	error,
) {
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
	for code := range settings {
		normalized := strings.ToUpper(code)
		if byCode[normalized] == nil {
			unknown = append(unknown, code)
		}
	}
	if len(unknown) != 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown analysis rule(s): %s", strings.Join(unknown, ", "))
	}

	configured := make(map[string]config.RuleConfig, len(settings))
	for code, setting := range settings {
		configured[strings.ToUpper(code)] = setting
	}
	registry := &Registry{settings: make(map[string]configuredRule, len(all)), root: root}
	for _, rule := range all {
		meta := rule.Meta()
		normalized := strings.ToUpper(meta.Code)
		setting := configured[normalized]
		if len(wanted) != 0 && !wanted[normalized] {
			continue
		}
		if len(wanted) == 0 && setting.Enabled != nil && !*setting.Enabled {
			continue
		}
		severity := meta.DefaultSeverity
		if setting.Severity != "" {
			severity = diagnostic.Severity(setting.Severity)
		}
		registry.rules = append(registry.rules, rule)
		registry.settings[meta.Code] = configuredRule{severity: severity, excludes: setting.Excludes}
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

func (registry *Registry) hasRule(code string) bool {
	for _, rule := range registry.rules {
		if rule.Meta().Code == code {
			return true
		}
	}
	return false
}

func (registry *Registry) needsSSA() bool {
	for _, rule := range registry.rules {
		if ssaRuleCodes[rule.Meta().Code] {
			return true
		}
	}
	return false
}

var ssaRuleCodes = map[string]bool{
	"argument-overwritten-before-use": true,
	"context-key-type": true,
	"dangerous-directory-removal": true,
	"discarded-pure-result": true,
	"duplicate-trim-cutset": true,
	"finalizer-captures-object": true,
	"impossible-interface-nil-comparison": true,
	"ineffective-value-receiver-assignment": true,
	"infinite-recursion": true,
	"invalid-listen-address": true,
	"invalid-regexp": true,
	"invalid-strconv-argument": true,
	"invalid-time-layout": true,
	"invalid-url": true,
	"invalid-utf8": true,
	"ip-byte-comparison": true,
	"leaky-time-tick": true,
	"misaligned-atomic-64": true,
	"nan-comparison": true,
	"never-nil-comparison": true,
	"nil-map-assignment": true,
	"non-pointer-unmarshal": true,
	"overlapping-encode-buffers": true,
	"overwritten-before-use": true,
	"pointless-integer-math": true,
	"possible-nil-dereference": true,
	"random-bound-one": true,
	"regexp-find-all-zero": true,
	"regexp-match-in-loop": true,
	"self-assignment": true,
	"sort-non-slice": true,
	"swapped-errors-is-arguments": true,
	"testing-fatal-in-goroutine": true,
	"timer-reset-drain-race": true,
	"unbuffered-signal-channel": true,
	"unchanged-loop-condition": true,
	"unexported-serialization-fields": true,
	"unsupported-binary-write": true,
	"unsupported-marshal-type": true,
	"unused-append-result": true,
	"writer-buffer-mutation": true,
	"zero-replacement-limit": true,
}

func allRules() []Rule {
	return[]Rule{
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
