Synthetic asynchronous stack-depth case table

APIs: 62.0 and 67.0. Routes: named Test.startTest/stopTest and standalone enqueue from anonymous Apex, with persisted Account observations.

capped_{plain,delay,options}_{1,2,3}: explicit maximum depth 2, chain lengths limit-1, limit, limit+1; three child submission overloads.
inherited_one: explicit maximum 1 with a rejected second job.
uncapped_{one,child,grandchild}: no explicit maximum; one, two and three queueables.
zero_cap: explicit zero-depth submission rejection twin.
future_{1,2,3}, batch_{1,2,3}: root plus attempted descendants; record accepted scheduling separately from executed callbacks.
mixed_four: queueable -> future -> batch finish -> queueable.
overflow_flush: uncaught inherited-depth rejection surfaced while stopTest flushes; standalone root status/message is read after dispatch.

Named assertion failures prefixed STACK transport observations. Other failures preserve exact exception type/message. Standalone rows contain completed callback observations plus root job status/errors; they do not infer outcomes from submission success.
