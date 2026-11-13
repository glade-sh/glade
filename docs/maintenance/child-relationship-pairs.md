# Child relationship pair check

The pairs generator checks each unordered pair of covered features and shape
markers against the union generated from the empty, single and full shapes.
It reports missing aliases and parent keys; it does not write a filter file.
Shards are zero-based. Each ordinal in the lexicographic pair enumeration is
assigned to `ordinal % n`, so all `n` shards of the same commit are required.
Without either shard variable the test checks all pairs. Set both variables;
invalid or incomplete settings fail the test. Each shard independently builds
the union before checking its pairs, and runs at most `GOMAXPROCS` pairs at a
time. Progress is logged every ten pairs and at the end. Use `-v` to stream it.

From the coordinator's shell, after committing the worktree, dispatch each
shard with the advisor runner. For example, with `n=4`, run each command in a
separate terminal or managed job and save its output and exit status:

```bash
n=4
# Repeat for i=0, 1, 2, 3. Use distinct logs and retain each exit status.
i=0
RTEST_HOST_ORDER=razor.local,casper.local RTEST_TETHER_STALE=14400 \
RTEST_ENV="GLADE_UPDATE_CHILD_REL_FILTER=pairs GLADE_CHILD_REL_PAIRS_SHARD=$i GLADE_CHILD_REL_PAIRS_SHARDS=$n" \
advisor/gates/rtest.sh "$WORKTREE" -v -count=1 -timeout=0 \
  -run '^TestChildRelationshipFilterGenerate$' ./internal/vm 2>&1 | tee "pairs-$i.log"
# With bash, record ${PIPESTATUS[0]} immediately; do not use tee's status.
```

The runner uses the committed HEAD, chooses an available host from the given
order, and keeps the remote job tethered to its caller. Apply the campaign host
capacity rules when scheduling. Do not use its separate `RTEST_SHARDS` option
for this focused test. Keep all callers alive and retain the logs and statuses.

To merge results, collect the logs and statuses for every index from `0` through
`n-1`, confirm the same commit and shard count, and verify each final progress
line reaches its shard total. Concatenate the logs for a combined report; sum
checked pairs and additions across the final lines. All shard exit statuses must
be zero, all expected shards must finish, the checked sum must equal the full
pair count printed by each shard, and the additions sum must be zero for a
clean result. Missing or failed shards must be rerun at the same commit and
count. There is no generated artifact to merge.
