package agent

// Why the last process lookup answered nothing.
//
// Lookup reports ok=false and stops there, which is the right shape for a
// caller deciding whether an agent is an ancestor: there is nothing to
// branch on beyond yes and no. It is the wrong shape for the person
// reading a log line afterwards. A denied /bin/ps, a pid that has already
// exited, and a host with no process reader at all are three different
// repairs, and all three arrive as the same false.
//
// So the reason is recorded beside the answer rather than folded into it.
// It is advisory: a caller that ignores it behaves exactly as before, and
// nothing here changes what a walk decides.

import "sync/atomic"

// lastLookupErr is the reason the most recent failed lookup gave. It is
// atomic because a walk may run from any goroutine, and a torn read of a
// diagnostic string is a worse outcome than a stale one.
var lastLookupErr atomic.Value // string

// noteLookupErr records why a lookup answered nothing. An empty reason is
// ignored, so a caller cannot erase a real one with a blank.
func noteLookupErr(reason string) {
	if reason == "" {
		return
	}
	lastLookupErr.Store(reason)
}

// clearLookupErr is called by a lookup that answered. The reason belongs
// to the last FAILURE, and a stale one read after a success names a
// problem that is no longer there.
func clearLookupErr() { lastLookupErr.Store("") }

// LookupError returns why the most recent process lookup answered nothing,
// or "" when the last one answered or none has run.
//
// Log it next to a detection that came back negative. A caller that treats
// "" as "the lookup worked" is correct: every failing path in this package
// records a reason before it returns.
func LookupError() string {
	reason, _ := lastLookupErr.Load().(string)
	return reason
}
