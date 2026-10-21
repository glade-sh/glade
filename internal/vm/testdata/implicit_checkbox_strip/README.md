# Implicit Checkbox stripping candidate

The runtime change and local proof are pending candidates, not a native oracle.
The ordinary custom record must be fresh and have a metadata-defaulted false
Boolean value that is neither explicit nor user-set. Only unscoped CREATABLE
stripping with effective field readability is covered. Queried, cloned, trigger,
persisted and intrinsically readonly or unreadable values remain on the prior stripping path.
The field's create/edit permission stays denied; root CRUD is still enforced.
There is no permission fallback or grant-matching change.

`proof-project/DefaultFieldScopeProbe` is the local fail-before/pass-after
regression plus explicit readonly, assigned writable and unassigned controls.
Constructor/default provenance and queried/cloned/trigger/intrinsic/read/scoped
controls are also covered by the Go test. These are local diagnostic assertions.

`probe-cases.json` and `native-project` retain the existing pending packet with
null expected values: eight constructor/JSON omission, explicit false/true,
writable-grant and unassigned shapes, at API 62/67/68 in anonymous and `@IsTest`
contexts, plain and parcel namespaces: 96 cells. Exact native stripping of an
implicit custom Checkbox has not been observed. Existing explicit text-grant
and standard loadDefaults observations do not settle this behavior.

Anonymous observations require the corresponding owned Standard User session.
`System.runAs` is test-only: anonymous setup creates the actor, then the actor
session invokes `readCurrent`. If sessions are unavailable, carry the 48
anonymous cells as ACTOR_SESSION_UNAVAILABLE. Never substitute an admin or test
execution. The independently decisive 48 test cells use the test-only helper.
Namespace observations require an authorized namespaced fixture; do not relabel
plain rows with a string prefix.

Capture full exceptions, describe facets, removed fields, modified indexes,
returned/original JSON, populated fields, isSet and direct values, for enforced
and unenforced root CRUD. Do not skip exceptions or infer omitted-constructor
answers from JSON or explicit-false controls. Native disagreement requires
narrowing or dropping the isolated candidate.
