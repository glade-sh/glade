# Clean-Room Policy

`glade` is a clean-room implementation of a local Apex runtime. Contributors
must use public documentation, public grammars, open file formats, owned test
fixtures, and black-box behavioral tests.

Do not copy proprietary source code, decompiled implementations, private data
structures, private protocol details, license enforcement details, or inferred
function internals into this project. Reverse-engineering notes may describe
public product behavior and compatibility gaps, but implementation work must be
based on independent design and public/owned inputs.

## Allowed Inputs

- Salesforce public documentation.
- Public Apex grammar projects with compatible licenses.
- Public open source dependencies with compatible licenses.
- Apex source, metadata, and fixture data owned by the contributor or provided
  under a compatible license.
- Black-box observations of Salesforce behavior using project-owned Apex and
  metadata.
- Public CLI help text and public documentation from comparable tools.

## Disallowed Inputs

- Proprietary source code.
- Decompiled or disassembled implementation logic.
- Private symbols, private package layouts, or private type layouts from
  proprietary binaries.
- License validation logic or bypass details from proprietary tools.
- Non-public customer data or org metadata without explicit permission.

## Compatibility Work

Compatibility fixtures should record the Apex source, metadata, seed data,
invocation, and expected externally observable behavior. The expected behavior
should come from Salesforce itself or from a written public specification, not
from proprietary implementation internals.

When runtime behavior is ambiguous, confirm it against the connected scratch org
before changing `glade`. Use `nu-dx-org` for small executable probes:

```bash
echo "System.debug('Hello from CLI');" | sf apex run --target-org nu-dx-org
```

Keep the probe minimal, record the observed result in the test or commit
context, and then encode the behavior in an owned regression or compatibility
fixture.

When the blocker is a missing system class, method, field, or object shape,
consult the public stubs under `example-projects/stubs` before patching. Use
them to fill the nearby public surface area, not just the single failing member,
while still keeping the implementation minimal and backed by scratch-org
behavior or an owned regression.
