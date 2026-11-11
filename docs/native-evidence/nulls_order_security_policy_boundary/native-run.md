# Native capture: inactive transaction security policy setup boundary

A fresh 30-day Developer scratch org was created from a campaign Dev Hub. Two synthetic Apex policy classes deployed, but deploying inactive, no-notification policy metadata was rejected for insufficient access. Queries at API 62 and 67 returned no policies. No policy was activated, no authentication action was performed and no event or notification was generated. This is a setup boundary, not an ordering result. The scratch org was deleted and ScratchOrgInfo status verified as Deleted.
