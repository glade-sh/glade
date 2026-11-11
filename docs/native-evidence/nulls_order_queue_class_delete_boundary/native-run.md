# Native capture: deleted queued-test class boundary

A fresh 30-day Developer scratch org was created from a campaign Dev Hub. The synthetic classes in ../nulls_order_test_class_delete_boundary/classes were deployed and run asynchronously, then one was deleted through metadata. The three captured ApexTestQueueItem IDs were re-queried at API 62 and 67: all ApexClassId values remained populated and distinct. Temporary IDs are not retained. This is a setup boundary, not a null-ordering result. The scratch org was deleted and ScratchOrgInfo status verified as Deleted.
