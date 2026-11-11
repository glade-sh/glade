# Native capture: appointment topic alternate lookup omission

A fresh 30-day Developer scratch org with LightningScheduler was created from a campaign Dev Hub. At API 62 and 67 synthetic tests omitted, rather than explicitly nulling, the unused alternate WorkType/WorkTypeGroup field. Group-backed controls succeeded and work-type-backed controls were rejected with INVALID_FIELD_FOR_INSERT_UPDATE; the required null-plus-populated ordering shape was not obtained. Tests rolled back and the scratch org was deleted, with ScratchOrgInfo status verified as Deleted.
