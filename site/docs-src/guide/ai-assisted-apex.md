---
pageType: guide
canonicalTask: /guide/ai-assisted-apex
---

# AI-assisted Apex with Glade

Use this prompt for any Apex feature, bug fix, or refactor. It makes the AI
agent work from the same local loop a developer would use: establish test
evidence, make the smallest source change, and rerun the same command.

## Before you start

Use an initialized Salesforce DX project and confirm the
[first local check](/guide/quickstart). Review your assistant provider's source
sharing and execution settings separately; using Glade does not change them.
The prompt keeps org-backed validation explicit and preserves existing changes.

## Where to put it

Paste the long prompt into a global skill, repository instruction file, or
agent memory. Good places include a user-level AI skill, a project
`AGENTS.md`, `CLAUDE.md`, or similar repository instruction file.

Use the prompt at the global level when most AI work touches Apex. Use it at
the repository level when only one Salesforce DX project needs the rule. Keep
project-specific commands outside the global prompt unless every project uses
the same test naming and branch shape.

## Global skill prompt

```text
Use Glade from the Salesforce DX project root when developing Apex.

This applies to any Apex feature, bug fix, refactor, trigger, service class,
selector, batch, queueable, schedulable, Visualforce controller, LWC Apex
controller, or Apex test change.

Default workflow:

1. Inspect the requested change, existing Apex tests, branch, and working tree.
   Choose the intended existing comparison ref and verify that it resolves.
   Replace <base-ref> below with that ref; do not assume a branch name.
2. Run local setup checks:
   mkdir -p reports
   glade doctor --project .
   glade config validate --project .
3. Establish the test evidence before editing production Apex.
   - For a behavior change, write the smallest failing Apex test first and
     verify it fails for the expected reason. For a bug, reproduce that bug.
   - For a behavior-preserving refactor, establish a passing baseline first.
   - Prefer one focused test method that names the behavior.
   - Use existing test factories and fixtures when they exist.
4. Run the narrowest Glade command for that failure or passing baseline:
   glade test --project . --class <TestClass> --method <TestMethod> --json --no-progress
5. Quote the exact command and result, including the failing diagnostic for a red test.
6. Make the smallest production change that satisfies the requested behavior.
7. Rerun the same focused Glade test until it passes.
8. Run source checks:
   glade check --project . --format json --output reports/glade-check.json --no-progress
9. Run affected tests before claiming success:
   glade test changed --project . --since <base-ref> --json --no-progress > reports/glade-test-changed.json
10. Read the saved check and test JSON. Report diagnostics and test summary
    counts: total, passed, failed, errors, skipped, and unsupported. An exit of
    0 with no selected tests is not test coverage; run an explicit relevant
    test or suite when the affected selection is empty.
11. Summarize the proof with commands, exit status, counts, and the changed Apex files.

Rules:
- Use Glade before a Salesforce deploy or org validation pass.
- Do not connect to a Salesforce org for the first pass unless the user asks.
- Check what Glade runs locally before treating unsupported platform services as bugs.
- If Glade reports an unsupported feature, name the unvalidated behavior and
  keep Salesforce as the validation gate. Unsupported test outcomes exit 1.
  Use only an explicitly authorized org for live validation.
- Do not rewrite valid Salesforce behavior to satisfy a Glade limitation. Report the mismatch and use authorized Salesforce validation for that path.
- Salesforce remains the validation gate for live auth, hosted service engines, deploy/retrieve, exact Lightning Experience behavior, Streaming, Pub/Sub, GraphQL, and exact production governor accounting.
- Do not invent one-off local escape hatches. Let the failing test and Glade diagnostic drive the change.
```

## Task prompt

Use this shorter prompt when you want the agent to apply the global rule to one
piece of work:

```text
Implement this Apex change with Glade.

Start from the Salesforce DX project root. For a behavior change, write the
smallest failing Apex test first and verify the expected failure. For a
behavior-preserving refactor, establish a passing baseline first. Use:

glade test --project . --class <TestClass> --method <TestMethod> --json --no-progress

Choose the intended existing comparison ref, verify it resolves, and replace
<base-ref> below. Then make the smallest source change, rerun the same focused
test, and run:

mkdir -p reports
glade check --project . --format json --output reports/glade-check.json --no-progress
glade test changed --project . --since <base-ref> --json --no-progress > reports/glade-test-changed.json

Read the saved JSON and report the exact commands, exit status, diagnostics,
and total, passed, failed, errors, skipped, and unsupported test counts. If the
affected selection is empty, run an explicit relevant test or suite. Name
unsupported Salesforce boundaries as unvalidated behavior, not passing proof.
Do not rewrite valid Salesforce behavior to satisfy a Glade limitation.
```

## How it should behave

Local test isolation is not an OS security sandbox. Core local checks do not
upload source to a Glade service, but custom code, plugins, imports, and an
external AI provider may use the network. Review what you authorize and do not
send proprietary source or credentials to a provider without permission.

The agent should come back with the test it added, changed, or reused; the
initial failure or passing refactor baseline; the source change; and the
commands and test counts at the end.

For a bug fix, the first failing test should reproduce the bug. For a new
feature, the first failing test should describe the smallest useful behavior.
For a refactor, the first Glade pass should preserve behavior before source
movement starts, then affected tests should run after the movement.

Use the [affected tests](/guide/affected-tests), [local testing](/guide/local-testing),
[automation and JSON](/guide/automation),
[Apex language compatibility](/reference/apex-language-compatibility), and
[support map](/guide/support-map) guides when the agent needs a narrower test
selector, machine-readable evidence, the reserved-identifier contract, or a
clear local runtime boundary.
