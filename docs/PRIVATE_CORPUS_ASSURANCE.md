# Private-Corpus Salesforce Assurance

This published v0.2.11 surface snapshot records how the exact Glade candidate
behaves against two authoritative repositories. Public artifacts identify them only as
`private-corpus-001` and `private-corpus-002`.

The result is surface-specific. It is not a claim of blanket Salesforce parity.
Compile readiness, real local test readiness, fresh Salesforce runtime parity,
and explicit non-parity are separate outcomes.

## Exact reviewed inputs

| Input | Identity |
| --- | --- |
| Glade commit | `b8a60258aa5c54777535d356e3ce1e746a874cd5` |
| Glade binary SHA-256 | `be4bd8cc105b1b84ee83d8dad491d0ede2a096038cd916ecb0e1fbadf68d0db0` |
| Glade Tools commit | `7cf3d717c266c06d51583c297593f579b8acc1b1` |
| Glade Tools ARM64 binary SHA-256 | `660bf89394094186379c1773d0e8aa2887cbff7f6681b67937c7218ccf256810` |
| Glade Tools AMD64 binary SHA-256 | `f3842194bd55476f4d0ef8a01ed79ed7757d7f617a4c2d4ba1e11bbcbc075252` |
| Frozen scope SHA-256 | `ec25212379b73f65320f7cd6b0c5d81638a98124da6f4713c5f8ca49e79587cf` |
| Receipt SHA-256 | `bf0a7b7a9fc2b0a7e505677b37c4891acea4f9b1cad8edb0c6ba714e3709517c` |
| Assurance JSON SHA-256 | `921bbc27c8fdc62e3e340138c26e1ea34b8137f206d251c66244bb63642aae04` |
| Original explorer export SHA-256 | `5bad30dfb04858f39d11c33a82e1290181d376ea58205a80ce47467eaff21625` |
| Packet manifest SHA-256 | `f9d9593c9b09c1c062bca5abb515b79f7fc7bcbbd350b835eddf70696c3e896e` |

The docs-only follow-up does not change the reviewed product candidate.

## Outcome

- 321 usage keys reconciled with zero unknown usage.
- 184 required surfaces derived from the sealed scope, profile, usage, and
  decision files.
- 178 compile-ready surfaces.
- 178 test-ready surfaces with required local proof.
- 54 runtime-parity-ready surfaces backed by fresh Salesforce execution.
- 107 explicit zero-credit non-parity outcomes: 101 local-contract-only rows
  and six separately authorized hosted waivers.
- Six hosted-deferred surfaces remain neither compile-ready, test-ready, nor
  runtime-parity-ready in this snapshot.

The readiness fields overlap. Do not add the counts together.

## Repository and Salesforce proof

Both repositories passed every required `glade check` and real `glade test`
shard on the exact candidate. The authoritative replay contains eight passing
records across local and remote hosts. Superseded resource-contention and
exhausted-hub attempts are retained outside the final receipt; their isolated
replacements passed and are the records used for readiness.

Fresh scratch-org proof closed all 77 planned Salesforce obligations: 23
compile observations and 54 runtime observations. The scratch orgs and both
remote attempt roots were removed, with cleanup receipts reporting no residue.

## How to use this result

Use [the assurance explorer](https://glade.sh/private-corpus-assurance.html)
to filter by namespace, repository, disposition, evidence, exclusion, or text.
Each row states whether it has compile, local-contract, runtime, or non-parity
evidence. Keep Salesforce in the release gate for hosted-deferred and explicit
non-parity behavior.

The current explorer uses the shared site styling and retains the exact assurance
JSON bytes identified above. The original self-contained HTML export and its
checksum remain available in the
[archived source](https://github.com/glade-sh/glade/blob/37c2bf878133cef2879124e4f47d20ba51d5bcfd/site/docs-src/public/private-corpus-assurance.html).
Presentation changes do not refresh the v0.2.11 evidence or its reviewed inputs.

The first-party Glade Tools assurance workflow owns scope freezing, usage
reconciliation, local proof, repository replay, Salesforce execution, cleanup,
and the acyclic receipt. Base Glade owns the compiler and runtime being tested.

## Tagged v0.2.12 product validation

This final tagged result validates v0.2.12 separately. It does not alter the
published v0.2.11 surface snapshot or its explorer.

| Input | Identity |
| --- | --- |
| Glade product commit | `3a454dee3cb35c604cb1bf25e6a8972b63dd7c81` |
| Glade binary SHA-256 | `9bd1d8efbeb53af707ec5df649103f3f462fc800410922ce54f3b89a67c5bf83` |
| Glade Tools commit | `18dd0e23cb540fdacdaaafa51b69c35d25426436` |
| Glade Tools binary SHA-256 | `b9805c4c5fadf1c8869810f685193f2f5d405bf836410059148e8c14ed565249` |

- `private-corpus-001`: check exit 0/0 diagnostics; tests 12,315/12,315 with
  0 failed/compile/runtime/unsupported.
- `private-corpus-002`: check exit 0/0 diagnostics; tests 782/782 with
  0 failed/compile/runtime/unsupported.
- Public corpus: 86 projects, 40 expected/40 observed diagnostics, zero
  missing/unexpected/unclassified, and an exact identity multiset match. Public
  diagnostics are the known baseline, not passes.
- Local Apex release gate: 9/9 composed checks pass.
- Salesforce tagged-release gate: 475/475 pass, zero fail/inconclusive, and
  cleanup PASS.

| Receipt | SHA-256 |
| --- | --- |
| Candidate release authority | `845206007a796ffb0235a555ce879d7daa5a1d2b4cfa05a8fe36e03424ddb1e2` |
| `private-corpus-001` authority | `118c8b1d5b7075ff22a90ffa5df2cd1fc1aeb445ef9f5312b2379ba9fec88335` |
| `private-corpus-002` authority | `61bb8f208128a184df750fe115f2852886d1e8376235e5c325771a6055e7dc10` |
| Final public-corpus receipt | `adb078a3d844ce3b4454a90185a1c978b0b2a828d48ada2c84e5333276fef8d8` |
| Local Apex release summary | `4b7e11bac77192605ea7dd5af33a7f3d10982cab61ee14fe37864e571e9af708` |
| Salesforce release authority | `c8829d8da76bc625c2f0056596fab5b31c3f68518844b94b7b0299982a11083c` |

The tagged validation is bounded to these exact receipts and does not claim
blanket Salesforce parity.
