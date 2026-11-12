# Native capture: deleted test class result boundary

A fresh 30-day Developer scratch org was created from a campaign Dev Hub. Three synthetic test classes were deployed and executed, then one synthetic class was deleted through metadata. The three previously captured result IDs were re-queried at API 62 and 67: all three ApexClassId values remained populated and distinct. The temporary IDs are not retained. This is a setup boundary, not a null-ordering result. The scratch org was deleted and ScratchOrgInfo status verified as Deleted.
