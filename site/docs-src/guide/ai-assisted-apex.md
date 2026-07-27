# AI-assisted Apex with Glade

Use this prompt for any Apex feature, bug fix, or refactor. It makes the AI
agent work from the same local loop a developer would use: test first, run
Glade, fix the smallest source slice, and prove the same command passes.

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
Use Glade from the SFDX project root when developing Apex.

This applies to any Apex feature, bug fix, refactor, trigger, service class,
selector, batch, queueable, schedulable, Visualforce controller, LWC Apex
controller, or Apex test change.

Default workflow:

1. Inspect the requested change and the existing Apex tests.
2. Run local setup checks:
   mkdir -p reports
   glade doctor
   glade config validate --project .
3. Write the smallest failing Apex test first.
   - Do not edit production Apex until a Glade test fails for the expected reason.
   - Prefer one focused test method that names the behavior.
   - Use existing test factories and fixtures when they exist.
4. Prove the red step with the narrowest Glade command:
   glade test --project . --class <TestClass> --method <TestMethod> --json --no-progress
5. Quote the exact command and the failing diagnostic.
6. Make the smallest production change that should pass that test.
7. Rerun the same focused Glade test until it passes.
8. Run source checks:
   glade check --project . --format json --output reports/glade-check.json --no-progress
9. Run affected tests before claiming success:
   glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
10. Summarize the proof with commands, exit status, and the changed Apex files.

Rules:
- Use Glade before a Salesforce deploy or org validation pass.
- Do not connect to a Salesforce org for the first pass unless the user asks.
- Check what Glade runs locally before treating unsupported platform services as bugs.
- If Glade reports an unsupported feature, say so and keep Salesforce as the validation gate.
- Salesforce remains the validation gate for live auth, hosted service engines, deploy/retrieve, exact Lightning Experience behavior, Streaming, Pub/Sub, GraphQL, and exact production governor accounting.
- Do not invent one-off local escape hatches. Let the failing test and Glade diagnostic drive the change.
```

## Task prompt

Use this shorter prompt when you want the agent to apply the global rule to one
piece of work:

```text
Implement this Apex change with TDD and Glade.

Start from the SFDX project root. Write the smallest failing Apex test first.
Prove it fails with:

glade test --project . --class <TestClass> --method <TestMethod> --json --no-progress

Then make the smallest source change, rerun the same focused test, run:

mkdir -p reports
glade check --project . --format json --output reports/glade-check.json --no-progress
glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json

Report the exact commands and diagnostics. Do not claim success until the Glade
commands pass or you have named the unsupported Salesforce boundary.
```

## How it should behave

The agent should come back with the test it added or changed, the command that
failed first, the production change, and the commands that passed at the end.

For a bug fix, the first failing test should reproduce the bug. For a new
feature, the first failing test should describe the smallest useful behavior.
For a refactor, the first Glade pass should preserve behavior before source
movement starts, then affected tests should run after the movement.

Use the [affected tests](/guide/affected-tests), [local testing](/guide/local-testing),
[Apex language compatibility](/reference/apex-language-compatibility), and
[support map](/guide/support-map) guides when the agent needs a narrower test
selector, the reserved-identifier contract, or a clear local runtime boundary.
