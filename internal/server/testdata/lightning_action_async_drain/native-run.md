# Lightning action async drain: native capture

Question: what happens to async work (Queueable, `@future`, platform event
publish) enqueued by an `@AuraEnabled` method called from a Lightning
component (LWC imperative, LWC wire and Aura all call the same
`ApexActionController.execute` action)?

## Setup

- Fresh 30-day scratch org (Developer edition, no features) on a campaign Dev
  Hub; deleted after the run. Salesforce CLI 2.157.8.
- Two copies of one controller, deployed at class API 62.0 and 67.0, each with
  its own platform event (PublishAfterCommit, one text field) and an
  `after insert` trigger on it. The controller source is mirrored by
  `asyncDrainController` in `internal/server/lightning_async_drain_test.go`.
- Every action first inserts an Account `D-<tag>` (the action's own DML).
  The Queueable, the future method and the event trigger each insert an
  Account (`Q-`, `F-`, `E-` prefix) whose Description records how many `D-<tag>`
  rows they could see (`seesAction=N`), which tells whether the action had
  committed when they ran.
- Calls were made as the scratch org's admin from a real Lightning Experience
  session, through the `/aura` endpoint with the page's own context and CSRF
  token (`aura://ApexActionController/ACTION$execute`). After each response the
  org was polled (SOQL on Account, every 4 s, up to 75 s, 150 s for the 50-job
  cases) until the row set was stable.
- Raw row timing is not recorded in org.tsv: async rows were first seen 4 to 15
  seconds after the response, events slowest. Only what both APIs share is
  recorded.

## Result (identical at API 62.0 and 67.0)

| case | caller sees | committed rows after settling |
| --- | --- | --- |
| enqQueueable | SUCCESS, return value, `Limits` queueable 1/50 | `D`, `Q` with seesAction=1 |
| enqFuture | SUCCESS, future 1/50 | `D`, `F` seesAction=1 |
| pubEvent | SUCCESS, `isSuccess()` true | `D`, `E` seesAction=1 |
| allAsync (all three) | SUCCESS | `D`, `Q`, `F`, `E`, each seesAction=1 |
| enqChain (queueable enqueues a queueable) | SUCCESS | `D`, `Q`, child `Q` ran |
| enqQueueableFail (job throws) | SUCCESS (the job fails later) | `D` only; the failed job's own insert is rolled back, job status Failed |
| allThenAura / allThenPlain (action throws after enqueuing all three) | ERROR with the exception message | nothing: no action DML, no job, no event |
| allThenCatch (action catches its own exception) | SUCCESS | `D`, `Q`, `F`, `E` (commit and drain as usual) |
| enqSavepoint (enqueue, then `Database.rollback` to a savepoint before it) | SUCCESS | `D` only: the rolled-back enqueue never runs |
| enqMany50 / futMany50 | SUCCESS, 50/50 | `D` plus 50 async rows |
| enqMany51 | ERROR `Too many queueable jobs added to the queue: 51` | nothing |
| futMany51 | ERROR `Too many future calls: 51` | nothing |
| enqCacheable (`cacheable=true`, no DML) | SUCCESS | the Queueable ran |
| noAsync (control) | SUCCESS | `D` only |

Accept controls: noAsync and the single-kind cases. Reject controls: the two
throwing actions, the savepoint rollback and the 51-job limit cases, where
nothing is committed or run.

## Not captured (parked)

- Order between several different async kinds enqueued in one action.
- A future that throws (a job failure was captured for a Queueable only).
- `Database.insertAsync` and other async DML from an action.
- Futures and events enqueued before a savepoint that the action then rolls
  back to.
- `Database.executeBatch`, `System.schedule` and `System.scheduleBatch` from
  an action. No rows: they stay rejected as UnsupportedFeature (also when a
  drained Queueable calls them), and nothing from the action is committed.
