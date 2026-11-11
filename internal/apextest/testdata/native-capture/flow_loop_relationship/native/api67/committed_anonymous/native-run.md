# Native run

## Observation

```text
api=67.0
flow_api=67.0
context=successful anonymous transaction
deployment_success=true
native_complete=true
rows=1
method=synthetic RouteProbe.observe; debug observation; successful executeAnonymous commits
boundary=empty, null parent and nonmatching seed are included in the observation
scratch_duration_days=30
scratch_deleted_verified=ScratchOrgInfo Deleted and no ActiveScratchOrg
```

## Follow-up

```text
api=67.0
flow_api=67.0
context=separate successful, failing and readback anonymous transactions
deployment_success=true
observation=loop result is recorded in org.tsv
rollback=insert synthetic trigger account then insert account with missing Name; required-field error
readback=count synthetic trigger accounts after failed transaction, four
scratch_duration_days=30
scratch_deleted_verified=ScratchOrgInfo Deleted and no ActiveScratchOrg
```
