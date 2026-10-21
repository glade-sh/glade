# Passive cache method dispatch

The synthetic project reproduces resolved platform cache calls that enter through
passive catalog declarations instead of the builtin call switch. Its unrelated
private inner type, namespace and source API are local routing controls. They do
not establish new native namespace precedence or hosted cache semantics.

The runtime bridge covers only Org.getPartition(String), Session.getPartition(String)
and Session.isAvailable(). All operations reuse the existing local cache engine.
Owned platform_cache_captured/no_partition.json and availability.json establish
partition errors, missing-default errors and synchronous Session availability at
APIs 62, 67 and 68. The Go tests assert these same outcomes through both entry paths.
Existing captured conformance remains the oracle; no capture source is changed.

The CLI project has three cache regression methods and an unchanged partition-key
qualification control. PlatformCache is an explicit local org feature. No implicit
hosted capacity, eviction, session lifetime or cross-process persistence is added.
Unknown methods, non-static/wrong-signature declarations, invalid partitions and
source-defined methods remain controls. Existing cache capture carries retain
their owners and are outside this dispatch repair.
