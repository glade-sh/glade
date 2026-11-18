# Native run

Captured from two newly created one-day Developer scratch orgs at API 62.0 and 67.0. Each org was deleted after use and deletion was verified. The synthetic fixtures cover Account and Contact external-ID denial, standard-user granted and ungranted controls, and Account granted controls. Anonymous execution used the fixture wrapped in a savepoint and rollback to isolate its data. Anonymous output was HTML-decoded before comparison. Each observation was compared with the corresponding checked-in native table.
