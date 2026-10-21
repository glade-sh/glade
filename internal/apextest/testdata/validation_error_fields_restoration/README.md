# Validation error field identity regression

These owned fixtures restore agreement between a validation rule's declared
field, `Database.Error.getFields()`, and the corresponding field describe name.
The rule's owning namespace and an exact declared qualified field key determine
the identity. There is no suffix search or invoking-org namespace fallback.

`ValidationFieldsRestoration` exercises INSERT, UPDATE, record-level errors,
already-qualified metadata and held error text. It verifies the caller retains
the assembled trailing LF while the stored LongTextArea removes it. This storage
behavior is independently native-backed by `longtext_boundary_observations.json`;
the namespace-field restoration itself is local regression evidence, not a new
Salesforce parity claim.

The Go harness runs plain and owning-namespace projects at API62/67/68, plus a
consumer of a source-backed dependency whose validation field is already
qualified. The DML test retains standard, record-level, already-qualified,
foreign-qualified, unknown, ownerless, unrelated-owner and local-only controls.
No native capture is required or invoked by these tests.
