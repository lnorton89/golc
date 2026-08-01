// observer_test.go covers observer.go's post-mutation observer seam
// (07-05-PLAN.md Task 1), and 08-08-PLAN.md Task 2's addition:
// PublishMutationEvent, the exported entry point a non-HTTP control
// surface (script, wails, cli) uses to reach the same audit/SSE pipeline
// the HTTP path (mutate.go's fireMutationObservers call) uses. This file
// lives in package api (white-box), mirroring audit_test.go's own
// package-api convention -- it never needs routecatalog's test-only
// bridge, unlike the external api_test package's coverage_test.go/
// mutate_test.go/events_test.go.
package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPublishMutationEventNotifiesRegisteredObserversInOrder proves
// PublishMutationEvent has identical semantics to the unexported
// fireMutationObservers it wraps: every registered observer is notified
// exactly once, in registration order, synchronously.
func TestPublishMutationEventNotifiesRegisteredObserversInOrder(t *testing.T) {
	ResetMutationObserversForTesting()
	t.Cleanup(ResetMutationObserversForTesting)

	var order []string
	RegisterMutationObserver(func(ev MutationEvent) { order = append(order, "first:"+ev.Route) })
	RegisterMutationObserver(func(ev MutationEvent) { order = append(order, "second:"+ev.Route) })

	PublishMutationEvent(MutationEvent{Route: "scene activate", Source: "script"})

	require.Len(t, order, 2, "observer notifications")
	require.Equal(t, []string{"first:scene activate", "second:scene activate"}, order, "expected registration-order notification")
}

// TestPublishMutationEventNeverPanicsWithNoObservers proves an unregistered
// (empty) observer set is a safe, silent no-op -- a script-issued call
// firing this seam before any observer is registered must never crash the
// caller.
func TestPublishMutationEventNeverPanicsWithNoObservers(t *testing.T) {
	ResetMutationObserversForTesting()
	t.Cleanup(ResetMutationObserversForTesting)

	PublishMutationEvent(MutationEvent{Route: "scene activate", Source: "script"})
}

// TestPublishMutationEventCarriesNonHTTPSource proves a caller-supplied
// Source (e.g. "script") reaches every observer unchanged -- the seam
// never overwrites or defaults Source to "http", so an audit row or SSE
// event correctly attributes a script-issued mutation to its real origin.
func TestPublishMutationEventCarriesNonHTTPSource(t *testing.T) {
	ResetMutationObserversForTesting()
	t.Cleanup(ResetMutationObserversForTesting)

	var seen MutationEvent
	RegisterMutationObserver(func(ev MutationEvent) { seen = ev })

	PublishMutationEvent(MutationEvent{
		Route: "scene activate", Actor: "script:Chase", Source: "script", CorrelationID: "run-1",
	})

	require.Equal(t, "script", seen.Source)
	require.Equal(t, "script:Chase", seen.Actor)
	require.Equal(t, "run-1", seen.CorrelationID)
}
