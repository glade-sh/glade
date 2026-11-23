# Flow create output assigned to a record lookup

Captured 2026-11-23 in a fresh 30-day Developer scratch org with HealthCloudAddOn, then deleted and verified gone. A record-triggered after-save Flow deployed at API 67.0 creates an Account and assigns the Create Records element's automatic output to an Account lookup on the triggering record. Anonymous and isolated Apex tests at 62.0 and 67.0 observed the lookup populated. The observation query uses SYSTEM_MODE so the same field is visible at both versions.

An earlier capture without SYSTEM_MODE observed the populated lookup at 62.0, but the 67.0 query reported the custom field unavailable to the running user. The final observation avoids drawing a field-permission conclusion from that difference.
