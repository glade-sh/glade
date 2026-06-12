# AI-Assisted Glade Blog Series Plan

Generated: 2026-06-12

This plan turns `docs/research/ai-assisted-glade-blog-source-index.md` into a publishable series. The source index names the records. This file names the posts, their order, their claims, and the evidence each post needs before drafting.

## Series Shape

Working series title: **Building Glade With AI**

Tagline: **A clean-room Apex runtime, built in public with ledgers, fixtures, model sessions, and a lot of proof.**

Audience:

- Salesforce developers who want a local Apex runtime.
- Engineers curious about AI-assisted software work beyond demos.
- Tool builders who care about tests, product boundaries, and taste.

House rule for every post:

- Open with a concrete artifact: a commit, prompt, failing command, scan count, screenshot, or test result.
- State what the human chose.
- State what the AI system did.
- Show the proof that kept the work honest.
- End on the next artifact, not a lecture.

Recommended structure:

- **Core series:** 8 posts.
- **Appendix posts:** 4 posts for deeper product and research lanes.
- **Capstone:** one essay after the series, built from the lessons that repeat.

## Core Series

### 1. The First Cut

Thesis: Glade began as a fast, concrete build, not as an AI novelty story.

Opener:

- First commit: `43b79047`, `2026-05-02T11:31:30-07:00`, subject `phases 0-8 work`.
- Current count at audit time: `1,185` commits.

Sections:

1. The enduring project aim: local Apex running for development and testing.
2. The early build pace: May 2 start, May 24 Codex record begins.
3. Why the blog is not about magic: the durable record is commits, sessions, plans, tests, and ledgers.
4. The working method: user sets product taste and constraints; AI does research, implementation, review, and repair.

Evidence to pull:

- `git log --reverse --format='%h %ad %an %s' --date=iso-strict | head -5`
- `git rev-list --count HEAD`
- `README.md`
- `docs/research/ai-assisted-glade-blog-source-index.md`
- Early Glade AI session archive, used only as internal source material with internal paths redacted.

Visual:

- Simple timeline from May 2 first commit to June 12 audit.

Draft note:

- Keep this short. It is the doorway, not the whole cabin.
- Call the project Glade in the origin story. The origin is the long-standing desire for local Apex execution.

### 2. The Ledger, Not the Vibe

Thesis: The strongest AI-assisted work used measurable ledgers instead of claims.

Opener:

- User instruction: `Use that temp SURFACE_LEDGER.json as truth.`

Sections:

1. What the Surface Ledger measured.
2. Why stale reports were not enough.
3. The June 8 closeout sprint: build `/tmp/glade`, refresh ledger, split squads, integrate fixtures, refresh again.
4. The result: `missingShape=6838` to `missingShape=6774`, and `explicitUnsupported=1047` to `1111`.
5. Why `explicitUnsupported` was not a shortcut: only true external, server-only, or product surfaces.

Evidence to pull:

- `/Users/matt/.codex/memories/rollout_summaries/2026-06-08T13-28-49-k3IK-glade_surface_ledger_parallel_vertical_closeout.md`
- `docs/SPRINT_LOG.md`
- Seven fixture files listed in the rollout summary.
- Final check command: `compat surface check --ledger /tmp/glade-surface-final-20260608-065253/SURFACE_LEDGER.json --max-parser-failures 0 --max-missing-shape 6774`

Draft artifacts:

- `docs/research/blog-source-packets/2026-06-12-ledger-not-vibe-source-packet.md`
- `docs/research/blog-drafts/2026-06-12-ledger-not-vibe-draft.md`

Visual:

- Before/after bar: missing shape down 64, explicit unsupported up 64.

Draft note:

- This is the best proof post. Write it early.

### 3. The AI Field Crew

Thesis: The project used AI less like one assistant and more like a field crew with a foreman.

Opener:

- User instruction: `This sprint MUST use parallel subagent squads to move faster.`

Sections:

1. The pattern: main thread plans and integrates; squads take separate verticals.
2. Codex example: Platform.Events, GraphQL/PubSub, external product surfaces.
3. Kilo/DeepSeek V4 Pro example: Workflow Task actions, ConnectApi NBA/Orchestrator, Aura/LWC controller evidence.
4. Cursor example: `29` subagent transcripts under one project.
5. The risk: parallel work only helps when each task has a tight fence.

Evidence to pull:

- Source index inventory.
- Kilo session ids:
  - `ses_15b3cd35affejsjHBzMmB5AkI9`
  - `ses_15b3c89d0ffeLs7WQaG8EalRpt`
  - `ses_15b3c4416ffe9RaMMDz98TW67Y`
- Cursor subagent transcripts: `/Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts/*/subagents/*.jsonl`
- Codex June 8 surface-ledger rollout.

Visual:

- Swimlane diagram: human request, main agent, three squads, final validation.

Draft note:

- Name Kilo as Kilo using DeepSeek V4 Pro.

### 4. Real Projects Made the Runtime

Thesis: Glade advanced fastest when real Salesforce projects broke it.

Opener:

- Kilo prompt: `review these blockers from repos. fix them all so the example projects are no longer blocked.`

Sections:

1. Why synthetic fixtures were not enough.
2. The repo-blocker loop: scan, group, fix generic runtime behavior, rerun.
3. Rules that prevented fake progress: no project-specific exceptions, no stubs, no scanner silence.
4. How failures turned into product code: Aura/LWC, workflow, Flow, ConnectApi, Visualforce, local tests.
5. What stayed visible: service-only leftovers and missing source stayed honest.

Evidence to pull:

- Kilo `ses_15b94df58ffeVfXNoLs6z6DpJk`
- Kilo `ses_15b4267a0ffeHDFnaK5EQCybeO`
- Codex post-parity memories and rollout summaries.
- Scanner artifacts in `/tmp` where still present.

Visual:

- Loop diagram: repo scan -> blocker group -> generic fix -> focused test -> fresh scan.

Draft note:

- This post should make the product feel earned.

### 5. Flow Was the Test

Thesis: Flow support became a clean example of turning a broad Salesforce feature into bounded AI work.

Opener:

- Kilo top session: `Implement P0 Flow TODO tasks`, `384` messages and `1,840` parts.

Sections:

1. Why Flow is broad: records, actions, connectors, faults, ordering.
2. How the work got divided: P0 tasks, subagent research, review passes.
3. What the human kept asking for: Salesforce-shaped behavior, not paper support.
4. How AI helped: gap cataloging, implementation, review, regression tests.
5. What this says about good AI tasks: make the domain big, but the packet small.

Evidence to pull:

- `docs/FLOW_TODO.md`
- Kilo `ses_157cec0adffefx6X6OCHHFqc4n`
- Cursor Flow subagents:
  - `Flow Fault Connectors`
  - `Flow Action Calls`
  - `Flow RecordUpdates`
  - `Flow Diagnostics Inventory`
- Relevant tests under `internal/automation`, `internal/dml`, and `internal/projectscan`.

Visual:

- Flow feature map with implemented, tested, and left-visible lanes.

Draft note:

- This can get too technical. Anchor every section on one failing case or one artifact.

### 6. When the Benchmark Lied

Thesis: Performance work only became useful after the team stopped trusting broad run time and chased exact hot paths.

Opener:

- A long run looked like the truth. A narrowed profile told a different story.

Sections:

1. The slow local-test problem.
2. Saved JSON as trailhead: private package artifacts plus open-source recipe artifacts.
3. Narrow repros before broad reruns.
4. What `pprof` showed: allocations, clone cost, alias walks, setup churn.
5. Durable wins: SOQL child-relationship caching, isolation journal, sentinel suites.

Evidence to pull:

- Codex local-test performance memory.
- Kilo `ses_152ffe691ffe1ldMyhEveudvpD`
- Cursor `564f9e70-b727-4728-9499-0bfcd50999a7`
- Commands and counts from the source index, with private package names and package-identifying counts withheld from public drafts.

Visual:

- Narrowing funnel: full suite -> saved artifact -> class -> method -> profile function.

Draft note:

- Keep the lesson concrete: profile first where the saved artifact points.

### 7. Taste Is a Requirement

Thesis: The product changed when the user stopped accepting a plain command line as good enough.

Opener:

- Cursor prompt: `this is very... blah.`
- Kilo prompt: `glade playground --help` returned `glade: unknown flag "--help"`.

Sections:

1. The first CLI worked, but it had no manners.
2. Help and output are product surfaces, not decorations.
3. Cursor lane: CLI personality, colors, formatting, and progress.
4. Kilo lane: help coverage and friendly command behavior.
5. The constraint: JSON, JUnit, and machine output must stay clean.

Evidence to pull:

- Cursor `977ccced-ccf6-4aad-b3a1-8e7f449b7539`
- Kilo `ses_156ac723affeKWcg674yS9DtEp`
- `docs/research/CLI_UX_DESIGN.md`
- `docs/superpowers/plans/2026-06-10-cli-ux-progress-system.md`
- `docs/superpowers/specs/2026-06-10-cli-personality-design.md`
- `internal/cliui`

Visual:

- Before/after CLI screenshots or terminal excerpts.

Draft note:

- This is the most human post. Use your direct prompts.

### 8. Keep the Tool Clean

Thesis: AI made it easy to grow code fast; product boundary work kept Glade from becoming a junk drawer.

Opener:

- User instruction: `I don't want ANY code in this project that isn't related to the actual functionality.`

Sections:

1. The tension: scanners and ledgers were useful, but not the deliverable.
2. The first split: maintenance commands out of base `glade`.
3. The final shape: `glade` as front door, first-party plugins underneath.
4. What stayed in base: runtime, parser, VM, CLI product commands.
5. What moved out: compat breadth, scanner dashboards, maintenance catalogs.

Evidence to pull:

- `docs/superpowers/plans/2026-06-11-glade-plugin-architecture.md`
- Codex plugin-boundary rollout summaries.
- `docs/PLUGINS.md`
- `site/docs-src/guide/plugins*.md`
- Tests around `glade plugins available` and base `glade compat` removal.

Visual:

- Boundary map: base Glade, first-party plugins, glade-tools source.

Draft note:

- This post gives the series its engineering spine.

## Appendix Posts

### 9. Local Visualforce and LWC

Thesis: Local Salesforce UI work required a bridge between source files, runtime state, and browser behavior.

Use this when the core series needs more product depth.

Evidence:

- Cursor LWC transcript `ddf8dd37-e79c-4d8c-98d6-c3e10261b17d`
- `docs/research/lwc-rendering-methodology.md`
- `docs/research/visualforce-local-rendering-methodology.md`
- `docs/superpowers/plans/2026-06-10-lightning-out-vf-lwc-runtime.md`
- `docs/superpowers/plans/2026-06-12-lwc-preview-playground.md`

Sections:

1. Why local UI is harder than local Apex execution.
2. Visualforce rendering plan.
3. LWC rendering plan.
4. What Glade can do locally.
5. What remains a browser or Salesforce container problem.

### 10. The Editor Surface

Thesis: VS Code became the live Glade editor surface because it matched local Apex workflows better than a second half-built IDE lane.

Evidence:

- Codex VS Code rollout summary.
- `docs/superpowers/plans/2026-06-12-vscode-extension-overnight-sprint.md`
- `contrib/vscode-glade`
- Bundled VSIX release smoke.

Sections:

1. JetBrains removal.
2. VS Code as Activity Bar, Test Explorer, CodeLens, LSP, and DAP.
3. Why `sfdx-project.json` became the activation boundary.
4. Release proof: bundled VSIX discovery.
5. What a real local-Apex editor surface still needs.

### 11. ISV Opportunity and the Scanner

Thesis: Glade's opportunity sharpened when AI research moved through real managed-package problems.

Evidence:

- Kilo `ses_153656a24ffeXoQhcUp7pjh2ug`
- Kilo performance scanner sessions.
- `docs/research/performance-scanner-roadmap.md`
- `docs/research/salesforce-managed-packages-performance.md`

Sections:

1. Managed packages as the hard case.
2. Local test speed as product value.
3. Performance scanning beyond static rules.
4. PR review and day-to-day developer loops.
5. Where Glade could fit.

### 12. What Needed a Human

Thesis: The repeated human contribution was taste, boundary-setting, and refusal to accept fake progress.

Evidence:

- Source index candidate hooks.
- User preference sections in Codex memory.
- Kilo and Cursor first user prompts.
- Review and repair sessions.

Sections:

1. The human set the product direction.
2. The human named proof before work began.
3. AI did useful volume work inside fences.
4. Review caught drift.
5. The method only worked when the artifact mattered more than the answer.

## Capstone

Working title: **What I Would Do Again**

Thesis: The useful pattern was not "AI wrote a product." The useful pattern was artifact-led work: exact prompts, exact gates, exact source files, and a human refusing to let the line wander.

Sections:

1. Start with a real artifact.
2. Split work only when boundaries are clean.
3. Make every AI claim pay rent with a test, ledger, or diff.
4. Keep product taste human.
5. Keep maintenance tools out of the product body.
6. Write the record while the work is warm.

Evidence:

- Use one small proof from each core post.

Ending:

- Return to May 2 and the first commit. Then show the June 12 source index. The distance between them is the story.

## Drafting Order

1. The Ledger, Not the Vibe
2. Taste Is a Requirement
3. The AI Field Crew
4. Real Projects Made the Runtime
5. When the Benchmark Lied
6. Keep the Tool Clean
7. Flow Was the Test
8. The First Cut
9. Capstone
10. Appendix posts as needed

Reasoning:

- Start with the strongest proof post.
- Follow with the most human product-taste post.
- Then explain the multi-agent method.
- Put the origin story after readers know why the project matters.

## Draft Checklist

Before writing each post:

- Pull 3 to 5 exact source passages or command outputs.
- Confirm dates and counts against the live store.
- Choose one screenshot, table, or diagram.
- Decide what not to include.
- Redact private package names from prompts, session titles, file names, and counts before drafting.
- Use Glade for early project references; redact internal repo paths and module paths from origin material.
- Keep transcript quotes short.
- Mark any claim from memory as needing live verification before publication.
