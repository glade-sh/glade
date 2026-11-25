# Subscriber visibility of a second private record

Captured on 2026-11-24 UTC in a fresh 30-day scratch org on a campaign Dev Hub. The org was deleted and deletion verified. The capture wrapper replaced the helper's hard-coded one-day creation argument with 30 days. Both the class and trigger reported API 62.0 and 67.0 in the respective test runs; see the fixture's `RUN-FACTS.txt` for timestamps and route.

The event contains the first record's ID. The trigger obtains a second private record's ID in system mode, then counts each record and each custom-reason share in user mode. Both records were created and owned by the publishing administrator. Two tests share each record to either its owner or another administrator. The outcome reports both parent counts, the two filtered share counts, and the unfiltered share count.
