# Native capture: appointment topic Field Service profile

A fresh 30-day Developer scratch org with LightningScheduler and FieldService:1 was created from a campaign Dev Hub. The synthetic Apex source is the unchanged source in ../nulls_order_appointment_omission/classes. At API 62 and 67 group-backed controls succeeded and work-type-backed controls were rejected with INVALID_FIELD_FOR_INSERT_UPDATE. No null-plus-populated ordering shape was obtained. Tests rolled back and the scratch org was deleted, with ScratchOrgInfo status verified as Deleted.
