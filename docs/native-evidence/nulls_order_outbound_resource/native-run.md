# Native capture: outbound resource lookup ordering

A fresh 30-day Developer scratch org was created from a campaign Dev Hub. API 62 reports the lookup absent. At API 67 a SeeAllData test selected a default template view and created two synthetic Campaigns and three synthetic resource records, with the lookup null once and set to two distinct Campaign IDs. It queried saved values and default and explicit NULLS ordering. The test transaction rolled back; no outbound operation was invoked. The scratch org was deleted and ScratchOrgInfo status verified as Deleted.
