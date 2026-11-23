# Before-save flow update of the triggering record with a filter

Captured 2026-11-23 in a fresh 30-day Developer scratch org, then deleted and verified gone. A record-triggered before-save Flow on Account (deployed at API 67.0) runs for created records and updates the triggering record with a filter that requires a source number to be non-null. At Apex API 62.0 and 67.0, anonymous Apex and an isolated test inserted one Account with a null source and one with a source. The record whose source was null kept its own value; the other took the source value. The observation queries use SYSTEM_MODE.

An earlier run without system-mode DML hit a field-access error at 67.0, so the final probe inserts as system at both versions. Update triggers and other filter operators were not captured.
