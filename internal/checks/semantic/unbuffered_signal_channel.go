//strider:ignore-file cognitive-complexity,identical-switch-branches,modifies-parameter
package semantic

import (
	"go/constant"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type unbufferedSignalChannelCheck struct{}

func (unbufferedSignalChannelCheck) Meta() Meta {
	return Meta{
		Code:            "unbuffered-signal-channel",
		Summary:         "detect unbuffered channels used for signal notification",
		Explanation:     "The os/signal package delivers notifications with non-blocking sends. An unbuffered channel can drop a signal whenever no receiver is immediately ready, so notification channels should have an appropriate buffer.",
		GoodExample:     "ch := make(chan os.Signal, 1)\nsignal.Notify(ch, os.Interrupt)",
		BadExample:      "ch := make(chan os.Signal)\nsignal.Notify(ch, os.Interrupt)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (unbufferedSignalChannelCheck) Run(pass *Pass) {
	calls := pass.argumentsByCallPosition()
	for _, call := range pass.staticCallsInPackage("os/signal") {
		if !isStaticFunction(call, "os/signal", "Notify") || len(call.Common().Args) == 0 || !isUnbufferedChannel(call.Common().Args[0]) {
			continue
		}
		node := explicitCallArgument(calls[call.Pos()], 0, call.Pos())
		pass.Report(node, "the channel used with signal.Notify is unbuffered and can drop signals")
	}
}

func isUnbufferedChannel(value ssa.Value) bool {
	for {
		switch converted := value.(type) {
		case *ssa.ChangeType:
			value = converted.X
		case *ssa.Convert:
			value = converted.X
		default:
			channel, ok := value.(*ssa.MakeChan)
			if !ok {
				return false
			}
			size := ssaConstant(channel.Size)
			if size == nil || size.Value == nil || size.Value.Kind() != constant.Int {
				return false
			}
			length, exact := constant.Int64Val(size.Value)
			return exact && length == 0
		}
	}
}
