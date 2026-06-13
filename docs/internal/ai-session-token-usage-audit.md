# AI Session Token Usage Audit

Date: 2026-06-13

This audit counts recorded token usage for the AI sessions tied to Glade. It
uses local logs only. It does not estimate missing usage from transcript lines.

## Sources Checked

- Codex JSONL sessions under `/Users/matt/.codex/sessions` and
  `/Users/matt/.codex/archived_sessions`.
- Kilo SQLite history at `/Users/matt/.local/share/kilo/kilo.db`.
- Cursor transcript folders under `/Users/matt/.cursor/projects`.
- Cursor global and workspace state databases under
  `/Users/matt/Library/Application Support/Cursor/User`.
- Cursor AI tracking database at `/Users/matt/.cursor/ai-tracking`.

Early project sessions are counted as early Glade. Old internal names and paths
are not used in this report.

## Counting Rules

Codex logs store `token_count` events with both cumulative totals and
`last_token_usage`. The archive contains repeated copies of some sessions, so
the count uses one canonical copy per session: the copy with the largest
recorded cumulative total or summed `last_token_usage`. Model totals then sum
the `last_token_usage` events from that canonical copy.

Kilo logs store token objects per assistant message. Kilo totals use the
recorded `tokens.total` when present, or the sum of input, output, reasoning,
and cache fields when `total` is absent. The user confirmed Kilo was DeepSeek
V4 Pro.

Cursor transcripts and state databases did not contain durable token totals.
Cursor entries are reported as transcript evidence only.

## Totals

| Source | Sessions | Calls/messages with tokens | Reported total tokens | Cached subset | Output tokens | Reasoning tokens |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Codex | 1,083 token-bearing sessions | 171,306 calls | 22,362,276,510 | 21,771,193,600 | 47,873,736 | 13,924,209 |
| Kilo | 76 sessions | 2,914 messages | 340,560,316 | 332,188,800 | 899,888 | 564,394 |
| Cursor | 63 transcript files | not available | not available | not available | not available | not available |

Recorded token-bearing total: **22,702,836,826 tokens**.

Cached subset: **22,103,382,400 tokens**.

Non-cached plus generated tokens: **599,454,426 tokens**.

## By Model

| Model | Source | Sessions | Calls/messages | Reported total tokens | Cached subset | Output tokens | Reasoning tokens | Span |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `gpt-5.5` | Codex | 1,063 | 159,370 | 21,061,211,804 | 20,511,064,192 | 44,130,800 | 12,041,382 | 2026-05-02 to 2026-06-13 |
| `gpt-5.3-codex` | Codex | 18 | 7,871 | 998,196,814 | 971,182,976 | 2,029,251 | 726,781 | 2026-05-16 to 2026-05-24 |
| DeepSeek V4 Pro | Kilo | 76 | 2,914 | 340,560,316 | 332,188,800 | 899,888 | 564,394 | 2026-06-07 to 2026-06-10 |
| `gpt-5.3-codex-spark` | Codex | 11 | 3,858 | 270,215,441 | 257,252,480 | 1,624,483 | 1,117,373 | 2026-06-08 to 2026-06-10 |
| `gpt-5.4` | Codex | 2 | 207 | 32,652,451 | 31,693,952 | 89,202 | 38,673 | 2026-05-16 to 2026-06-11 |

Cursor Glade transcripts mention `composer-2.5-fast` twice and `fast` once, but
no Cursor token counters were found for those model labels.

## By Project Era

| Era | Source | Sessions | Calls/messages | Reported total tokens | Cached subset | Output tokens | Reasoning tokens | Span |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Early Glade | Codex | 488 | 91,302 | 12,071,936,782 | 11,803,088,768 | 21,391,748 | 5,554,396 | 2026-05-02 to 2026-05-24 |
| Glade | Codex | 595 | 80,004 | 10,290,339,728 | 9,968,104,832 | 26,481,988 | 8,369,813 | 2026-05-24 to 2026-06-13 |
| Glade | Kilo | 76 | 2,914 | 340,560,316 | 332,188,800 | 899,888 | 564,394 | 2026-06-07 to 2026-06-10 |

## Cursor Findings

Cursor has useful process evidence, but not token totals in the local stores
checked here.

- Main Glade transcript folder: 39 JSONL files, 1,773 lines.
- Temporary Glade transcript folder: 1 JSONL file, 145 lines.
- Early Glade transcript folder: 23 JSONL files, 655 lines.
- Combined Cursor transcript evidence: 63 JSONL files, 2,573 lines.
- Transcript JSON objects had no `tokens`, `usage`, or `cost` keys.
- Global Cursor composer headers had 97 composer headers and
  `contextUsagePercent` on 74 of them. That field is window fullness, not token
  spend.
- Cursor AI tracking records model labels for generated code attribution, but
  no token totals.

## Read For The Blog

The headline number is not a guess: the local token-bearing logs record about
**22.7 billion tokens** across Codex and Kilo for the Glade build history found
here.

The shape matters more than the raw number. About **22.1 billion tokens** are
recorded as cached input or cache reads. The work was not one prompt and one
answer. It was a long-running local build loop where the same project context
kept getting pulled back into the model.

Codex carried the bulk of the token-bearing build history. Kilo adds a dense
DeepSeek V4 Pro pass in June. Cursor adds useful transcript evidence about the
workflow, but the local Cursor stores do not preserve token accounting.

## Limits

- Cursor cloud-side usage, if any, is not present in the local logs checked.
- Codex sessions without `token_count` events are counted as sessions but not as
  token usage.
- Kilo user and metadata rows without token objects are not included in token
  totals.
- The Codex archive has repeated session copies. The canonical-session rule is
  meant to avoid double-counting those copies.
