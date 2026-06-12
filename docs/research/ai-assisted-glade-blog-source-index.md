# AI-Assisted Glade Blog Source Index

Generated: 2026-06-12

This index maps the available AI-session record into blog-series material for the story of building Glade with AI help. It is a source map, not a finished outline. Each row names the likely post, the strongest archive paths, the tool or model lane, and the proof that makes the story more than a memory.

## Audit Inventory

| Source | Path | Span | Count | Notes |
| --- | --- | --- | ---: | --- |
| Git history | `/Users/matt/Dev/glade/.git` | 2026-05-02 onward | 1,185 commits on current HEAD | First commit is `43b79047` at `2026-05-02T11:31:30-07:00`, subject `phases 0-8 work`. |
| Codex raw sessions | `/Users/matt/.codex/sessions`, `/Users/matt/.codex/archived_sessions` | 2026-05-24 to 2026-06-12 | 217 Glade cwd sessions | Best source for full turn-by-turn work after May 24. |
| Codex memories | `/Users/matt/.codex/memories/MEMORY.md` | 2026-05-24 to 2026-06-12 | 26 Glade task groups | Best source for themes, counts, and reusable decisions. |
| Codex rollout summaries | `/Users/matt/.codex/memories/rollout_summaries` | 2026-05-25 to 2026-06-12 | 25 Glade summaries | Best source for clean story beats and final proof commands. |
| Kilo | `/Users/matt/.local/share/kilo/kilo.db` | 2026-06-07 to 2026-06-10 | 76 sessions, 3,063 messages, 14,521 parts | User confirmed this lane was DeepSeek V4 Pro. Strongest source for repo blockers, Flow, CLI help, and overnight sprint work. |
| Cursor transcripts | `/Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts` | mostly 2026-06-07 to 2026-06-10 | 39 transcripts, 1,773 lines | 10 top-level agent transcripts and 29 subagent transcripts. Strongest source for CLI taste, LWC, Visualforce, and strategy threads. |
| Cursor terminal captures | `/Users/matt/.cursor/projects/Users-matt-Dev-glade/terminals` | mostly 2026-06-07 to 2026-06-10 | 20 terminal files | Useful for proof, not narrative by itself. |
| Cursor screenshots | `/Users/matt/.cursor/projects/Users-matt-Dev-glade/assets` | 2026-06-10 | 2 screenshots | Likely useful for a visual post around CLI or UI work. |
| Cursor headers | `/Users/matt/Library/Application Support/Cursor/User/globalStorage/state.vscdb` | 2026-06-07 to 2026-06-10 | 97 headers, 21 Glade-named | Header list shows titles and line counts. Bodies are better in `.cursor/projects`. |
| Claude side traces | `/Users/matt/.claude/projects` | mixed | secondary | Found Glade references inside another Claude project store, but not a clean native Glade project session set. Treat as supporting material only. |
| Early Glade Codex raw sessions | internal early Glade path | 2026-05-02 to 2026-05-23 | 536 sessions | Internal-only origin material. Treat these as Glade sessions. Do not publish internal paths or module paths. |
| Early Glade Cursor transcripts | internal early Glade path | early project period | 23 transcripts, 632 lines | Internal-only origin material. Treat these as Glade transcripts. Do not publish internal paths or module paths. |
| Repo research docs | `/Users/matt/Dev/glade/docs/research` | 2026-06 | 6 files | Good edited source for CLI UX, LWC, Visualforce, performance scanner, and ISV themes. |
| Repo plans/specs | `/Users/matt/Dev/glade/docs/superpowers/plans`, `/Users/matt/Dev/glade/docs/superpowers/specs` | 2026-06 | 26 listed files | Good source for how prompts became executable work packets. |

## Candidate Posts

| # | Working title | Core question | Primary sources | Tool lane | Hook or quote | Proof to cite |
| ---: | --- | --- | --- | --- | --- | --- |
| 1 | The First Cut | How fast did the project go from first commit to working product shape? | Git history, Codex session count, repo docs | Git, Codex | `phases 0-8 work` | First commit `43b79047`, `1,185` commits by 2026-06-12, Codex sessions start May 24. |
| 2 | The Ledger, Not the Vibe | How did Glade avoid hand-wavy AI progress? | `/Users/matt/.codex/memories/rollout_summaries/2026-06-08T13-28-49-k3IK-glade_surface_ledger_parallel_vertical_closeout.md` | Codex | `Use that temp SURFACE_LEDGER.json as truth.` | Baseline `missingShape=6838`, final `missingShape=6774`, net `+64 explicitUnsupported / -64 missingShape`, final check with `--max-missing-shape 6774`. |
| 3 | The AI Field Crew | What did parallel AI squads change about the work? | Codex rollout summaries, Kilo squad sessions, Cursor subagent transcripts | Codex, Kilo DeepSeek V4 Pro, Cursor | `This sprint MUST use parallel subagent squads to move faster.` | Codex three-squad sprint, Kilo `Squad A/B/C`, Cursor `29` subagent transcripts. |
| 4 | Repo Blockers as a Workbench | How did failing real projects shape runtime support? | Kilo `ses_15b94df58ffeVfXNoLs6z6DpJk`, Kilo post-parity sessions, Codex post-parity memories | Kilo DeepSeek V4 Pro, Codex | `review these blockers from repos. fix them all...` | Kilo blocker sessions, scanner JSON prompts, before/after blocker counts in rollout summaries. |
| 5 | Flow Was the Test | How did Flow support become a good AI work packet? | Kilo `ses_157cec0adffefx6X6OCHHFqc4n`, `docs/FLOW_TODO.md`, Cursor Flow subagents | Kilo DeepSeek V4 Pro, Cursor | `Implement P0 Flow TODO tasks` | Top Kilo session by size: `384` messages, `1,840` parts. Cursor Flow Fault, Action, RecordUpdate subagents. |
| 6 | When the Benchmark Lied | How did performance work move from broad waits to exact hot paths? | Codex local-test performance memories, Kilo private-package profiling session, Cursor `Test command performance analysis` | Codex, Kilo DeepSeek V4 Pro, Cursor | `do a deep dive and profile of the long running tests` | Saved JSON artifacts, `pprof`, and private-corpus gates. Do not publish private package names or package-identifying counts. |
| 7 | Taste Is a Requirement | How did the CLI move from plain output to a product surface? | Cursor `977ccced-ccf6-4aad-b3a1-8e7f449b7539`, Kilo `ses_156ac723affeKWcg674yS9DtEp`, `docs/research/CLI_UX_DESIGN.md` | Cursor, Kilo DeepSeek V4 Pro | `this is very... blah.` | Cursor header records `CLI user experience improvement` with `2,204` added lines. Kilo prompt shows `glade playground --help` failing. |
| 8 | Local Visualforce and LWC | What did "local Salesforce UI" mean in practice? | Cursor LWC and Visualforce transcripts, `docs/research/lwc-rendering-methodology.md`, `docs/research/visualforce-local-rendering-methodology.md` | Cursor, Kilo DeepSeek V4 Pro | `Visualforce local rendering methodology` | Cursor top transcript `ddf8dd37...` has `344` lines. Research docs preserve the design argument. |
| 9 | Product Boundary Work | How did Glade stay the deliverable instead of a shed full of tools? | Codex plugin-boundary memories, `docs/superpowers/plans/2026-06-11-glade-plugin-architecture.md` | Codex | `I don't want ANY code in this project that isn't related to the actual functionality.` | `glade plugins available`, base `glade compat` removal, first-party plugin split, focused tests and smoke. |
| 10 | The Editor Surface | Why did VS Code become the live editor lane? | Codex VS Code rollout, `contrib/vscode-glade`, `docs/superpowers/plans/2026-06-12-vscode-extension-overnight-sprint.md` | Codex | `remove jetbrains plugin. just keep vs code.` | JetBrains removal, Activity Bar/Test Explorer/CodeLens/LSP/DAP, bundled VSIX smoke. |
| 11 | ISV Opportunity and the Scanner | How did customer-shaped Salesforce projects point the product? | Kilo `Salesforce ISV opportunity research`, Kilo performance scanner sessions, `docs/research/performance-scanner-roadmap.md` | Kilo DeepSeek V4 Pro | `find areas of opportunity for glade to help improve things` | Kilo research sessions, local Salesforce docs scans, enterprise project reviews. |
| 12 | What Needed a Human | Where did AI help, and where did human taste and boundaries matter? | Memory preference sections, Kilo and Cursor first user prompts, review sessions | All lanes | `No stubs. Do not silence the scanner.` | Repeated proof requirements: fresh baselines, exact counts, green gates, review passes, and final artifacts. |

## High-Value Raw Sources

### Codex

- `/Users/matt/.codex/memories/MEMORY.md`
- `/Users/matt/.codex/memories/rollout_summaries/2026-06-08T13-28-49-k3IK-glade_surface_ledger_parallel_vertical_closeout.md`
- `/Users/matt/.codex/memories/rollout_summaries/2026-06-12T03-20-13-U9bn-glade_vscode_local_apex_extension_and_jetbrains_removal.md`
- `/Users/matt/.codex/memories/rollout_summaries/2026-06-11T01-36-24-v2va-glade_plugin_architecture_plan_and_boundary_correction.md`
- `/Users/matt/.codex/memories/rollout_summaries/2026-06-11T03-54-26-eJC4-glade_plugin_available_discovery.md`
- `/Users/matt/.codex/memories/rollout_summaries/2026-05-27T03-50-31-5CWo-glade_local_test_performance_main_merge.md`

### Drafted Post Artifacts

- `docs/research/blog-source-packets/2026-06-12-ledger-not-vibe-source-packet.md`
- `docs/research/blog-drafts/2026-06-12-ledger-not-vibe-draft.md`

### Kilo / DeepSeek V4 Pro

Query the store by `project.worktree = '/Users/matt/Dev/glade'`.

- `ses_157cec0adffefx6X6OCHHFqc4n` - `Implement P0 Flow TODO tasks`
- `ses_156ac723affeKWcg674yS9DtEp` - `CLI usability and help command improvements`
- `ses_15a9e30c2ffegrHLVVv0HqGNDj` - `Glade repo unattended overnight sprint`
- `ses_15b3c89d0ffeLs7WQaG8EalRpt` - `Squad B: ConnectApi NBA+Orchestrator (@general subagent)`
- `ses_15b4267a0ffeHDFnaK5EQCybeO` - `Glade post-parity compatibility implementation`
- `ses_153656a24ffeXoQhcUp7pjh2ug` - `Salesforce ISV opportunity research`
- `ses_152ffe691ffe1ldMyhEveudvpD` - private-package test profiling and performance analysis
- `ses_14d52902cffe4TWZ9GAQdI6GOI` - `LWC Rendering Methodologies Research`
- `ses_14db4144bffeCVp7mb7K2LVstW` - `Visualforce local development with Glade`

### Cursor

- `/Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts/977ccced-ccf6-4aad-b3a1-8e7f449b7539/977ccced-ccf6-4aad-b3a1-8e7f449b7539.jsonl` - CLI user experience improvement.
- `/Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts/ddf8dd37-e79c-4d8c-98d6-c3e10261b17d/ddf8dd37-e79c-4d8c-98d6-c3e10261b17d.jsonl` - LWC rendering methodology.
- `/Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts/564f9e70-b727-4728-9499-0bfcd50999a7/564f9e70-b727-4728-9499-0bfcd50999a7.jsonl` - Test command performance analysis.
- `/Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts/1ce987d7-68fc-474d-8ff6-6adb99ce37b6/1ce987d7-68fc-474d-8ff6-6adb99ce37b6.jsonl` - Glade project blocker strategy.
- `/Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts/0956b2ab-3058-41dc-b130-f28fb913e72c/0956b2ab-3058-41dc-b130-f28fb913e72c.jsonl` - Salesforce platform functionality plan.
- `/Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts/*/subagents/*.jsonl` - subagent bodies for Flow, ConnectApi, external surfaces, deep dives, and reviews.

## Repro Commands

```bash
git log --reverse --format='%h %ad %an %s' --date=iso-strict | head -5
git rev-list --count HEAD

rg -l '^\{"timestamp":"[^"]+","type":"session_meta","payload":\{"id":"[^"]+","timestamp":"[^"]+","cwd":"(/Users/matt/Dev/glade|/tmp/glade|/private/tmp/glade)' \
  /Users/matt/.codex/sessions /Users/matt/.codex/archived_sessions | sort -u | wc -l

rg '^# Task Group: glade ' /Users/matt/.codex/memories/MEMORY.md | wc -l
find /Users/matt/.codex/memories/rollout_summaries -type f -name '*glade*' | wc -l

sqlite3 -header -column /Users/matt/.local/share/kilo/kilo.db \
  "select count(distinct s.id) as sessions,count(distinct m.id) as messages,count(distinct p.id) as parts
   from session s
   left join message m on m.session_id=s.id
   left join part p on p.session_id=s.id
   where s.project_id=(select id from project where worktree='/Users/matt/Dev/glade');"

find /Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts -type f -name '*.jsonl' | wc -l
find /Users/matt/.cursor/projects/Users-matt-Dev-glade/agent-transcripts -type f -name '*.jsonl' | xargs wc -l | tail -1
```

## Gaps And Cautions

- The earliest project period, May 2 through May 23, is covered by the early Glade AI-session set, not the later Glade-named path. Use it as Glade history.
- Kilo sessions have some blank per-session `agent` or `model` columns, but the user confirmed the Kilo lane was DeepSeek V4 Pro.
- Cursor global headers are useful for titles and added-line counts. The transcript bodies in `.cursor/projects/Users-matt-Dev-glade` are better for quotes.
- Claude has Glade references in a different project store. Do not treat those as the main Glade session archive without more verification.
- Prefer quoting user prompts and final proof lines. Avoid pasting long raw transcript blocks into public drafts.
- Do not publish private package names from the test corpus. Use `private managed-package corpus`, `private package gate`, or `private saved test artifact` instead. Open-source projects and repos are fine.
- Frame the origin around the long-standing goal of local Apex execution.
- Early Glade AI session history fills much of the May 2-23 gap. Mine it as internal source material. In drafts, name the project Glade and omit internal paths and module paths.
