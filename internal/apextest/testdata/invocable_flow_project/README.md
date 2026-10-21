# Synthetic project-loaded Invocable Flow fixtures

The captured fixture matrix has 168 answers across APIs 62.0, 67.0 and 68.0,
with independently captured anonymous and named routes. The normal project
loader discovers the synthetic Flow XML, helper classes and trigger metadata;
the shared runner registers the project runtime including every trigger event.
Named chunk envelopes are replaced only as transport, preserving each source
operation and comparing the complete reassembled native payload.

The matrix asserts before/after insert/update/delete input isolation, returned
record identity and caller writeback, null assignments, result flags and errors,
missing-output caller exceptions, fault/addError database outcomes, declared
uppercase record variables, legacy entry metadata and Flow-created triggers.
Global-null metadata is rejected; its unchanged modern runtime twin is checked.
Twelve named answers are labelled recovered after an exact closing-anchor repair
verified against the original whole-payload digest. The other 156 answers remain
observed. At APIs 67/68, 24 Flow-fault payloads compare the stable message parts
with the native SOAP guide anchor checked; API62 retains exact plain guide text.

The original local fixture retains two additional caller-input variations:
a before-update record whose Name was changed by its caller, and an after-insert
record with a distinct original Name. Both remain explicitly
**uncaptured: needs native row** and add no conformance cells.
API59 and activation/deactivation transitions have no asserted behavior here.
