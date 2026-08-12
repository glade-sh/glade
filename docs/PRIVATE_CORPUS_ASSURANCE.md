# Private-Corpus Salesforce Assurance

This release snapshot records how the exact Glade candidate behaves against two
authoritative repositories. Public artifacts identify them only as
`private-corpus-001` and `private-corpus-002`.

The result is surface-specific. It is not a claim of blanket Salesforce parity.
Compile readiness, real local test readiness, fresh Salesforce runtime parity,
and explicit non-parity are separate outcomes.

## Exact reviewed inputs

| Input | Identity |
| --- | --- |
| Glade commit | `b8a60258aa5c54777535d356e3ce1e746a874cd5` |
| Glade binary SHA-256 | `be4bd8cc105b1b84ee83d8dad491d0ede2a096038cd916ecb0e1fbadf68d0db0` |
| Glade Tools commit | `07663fff635bb6ab5feafdee8380f0afcdce0b92` |
| Glade Tools binary SHA-256 | `2796d6e4354d98305bb426274efc2b9f07609be60a8b2a3cde453225464368b4` |
| Frozen scope SHA-256 | `ec25212379b73f65320f7cd6b0c5d81638a98124da6f4713c5f8ca49e79587cf` |
| Receipt SHA-256 | `e2c455154681ab707014c113df30e748961916be53e478078bf09545d2c08496` |
| Assurance JSON SHA-256 | `921bbc27c8fdc62e3e340138c26e1ea34b8137f206d251c66244bb63642aae04` |
| Explorer SHA-256 | `5bad30dfb04858f39d11c33a82e1290181d376ea58205a80ce47467eaff21625` |
| Packet manifest SHA-256 | `cbbef68f5e11369b531963ddfc55664e289203345b0886b6a2fa507910c9c5d7` |

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
records across local and remote hosts. A superseded contention timeout is
retained outside the final receipt; the isolated replacement passed and is the
record used for readiness.

Fresh scratch-org proof closed all 77 planned Salesforce obligations: 23
compile observations and 54 runtime observations. The scratch orgs and both
remote attempt roots were removed, with cleanup receipts reporting no residue.

## How to use this result

Use [the self-contained explorer](https://glade.sh/private-corpus-assurance.html)
to filter by namespace, repository, disposition, evidence, exclusion, or text.
Each row states whether it has compile, local-contract, runtime, or non-parity
evidence. Keep Salesforce in the release gate for hosted-deferred and explicit
non-parity behavior.

The first-party Glade Tools assurance workflow owns scope freezing, usage
reconciliation, local proof, repository replay, Salesforce execution, cleanup,
and the acyclic receipt. Base Glade owns the compiler and runtime being tested.
