Fresh 30-day Developer scratch org, campaign Dev Hub, 2026-11-09.
Synthetic project: process_builder_shapes/project overlaid with project/ in this directory.
FlowSettings enabled active flow deployment; source deployed at API 67.0; Fern_Access assigned.
Named tests FernCases62Test and FernCases67Test ran at API 62.0 and 67.0; their sentinel assertion carries the observation.
Anonymous executions called the corresponding FernCases class at each API; debug marker captures the observation, then assertion rolls back.
aba: three interviews update one parent with A, B, A. Target triggers show A and B only; final value B.
splitfail: mixed cohorts reach different Apex actions; both input orders and a successful cohort are recorded. After-save action and survivor retries are observable.
Error ID is masked; output otherwise reproduces returned text. Scratch deletion returned success. The org identity matched both original named-test receipts and exactly one Dev Hub ScratchOrgInfo record, whose Status=Deleted; the matching ActiveScratchOrg count was zero.

flowafter: a second fresh 30-day scratch org captured a failing flow-action cohort plus a record outside that cohort. Its after-save flow runs for the survivor before the partial-save retry and again on retry. Both orgs were deleted; the second ScratchOrgInfo record was tracked before deletion and verified Status=Deleted, with no matching ActiveScratchOrg.
