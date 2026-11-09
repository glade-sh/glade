package vm

import "testing"

// An enqueued Queueable's AccessLevel members detach from the constants, but
// fields that held one AccessLevel still hold one object after the enqueue:
// async_access_boundaries transport rows observe aliasSame and repeatedSame
// true with queueSame false at API 62 and 67.
func TestDetachAccessLevelsKeepsAliasesWithinTheQueueable(t *testing.T) {
	for _, mode := range []string{"USER_MODE", "SYSTEM_MODE"} {
		t.Run(mode, func(t *testing.T) {
			original := accessLevelValue(mode)
			job := Object("AccessJob")
			job.Fields["direct"] = original
			envelope := Object("Envelope")
			envelope.Ref = newValueRef()
			envelope.Fields["levels"] = List(original, original)
			job.Fields["nested"] = envelope
			queued := cloneValueDetachedPreserveRefs(job)
			detachAccessLevels(&queued, make(map[uint64]bool))
			direct := queued.Fields["direct"]
			levels := queued.Fields["nested"].Fields["levels"].List
			if valueIdentityEqual(direct, original) || direct.Equal(original) {
				t.Fatal("a queued AccessLevel must not be the constant (queueSame false)")
			}
			if !valueIdentityEqual(direct, levels[0]) {
				t.Error("direct and nested references lost their shared identity (aliasSame)")
			}
			if !valueIdentityEqual(levels[0], levels[1]) {
				t.Error("repeated list entries lost their shared identity (repeatedSame)")
			}
		})
	}
}
