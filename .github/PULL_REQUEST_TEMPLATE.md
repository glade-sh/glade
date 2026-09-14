## What changes for a user?

Describe the problem, the smallest useful change, and the related issue when one exists.

## Scope and ownership

Name the affected files or interface. Identify overlap with other work and any generated output whose owning source must change. Do not replace unrelated local work.

## Evidence

| Check | Exact command or review | Version or commit | Result |
| --- | --- | --- | --- |
| Focused validation | | | |

Label evidence as source inspection, static check, local runtime, deterministic harness, or authorized Salesforce comparison. List skipped checks and why. Do not record a result as passed unless it ran against this change.

For a test-selection change, include selected and executed counts. For visible UI changes, include sanitized desktop and mobile screenshots plus the relevant loading, error, empty, focus, and recovery states.

## Compatibility, privacy, and risk

Explain any changed local boundary, data access, network request, persisted state, command side effect, or public support claim. State whether the change can merge independently and how to revert it.

## Review checklist

- [ ] The change is focused and preserves unrelated work.
- [ ] Commands and examples match the intended version; zero tests are not described as passing execution.
- [ ] No credentials, private source, customer records, or unredacted logs are included.
- [ ] Documentation and capability claims match the evidence; no unverified parity or completeness claim is introduced.
- [ ] Dependencies, owner decisions, and unverified checks are listed above.

Security-sensitive details belong in the project's private reporting channel rather than this public pull request.
