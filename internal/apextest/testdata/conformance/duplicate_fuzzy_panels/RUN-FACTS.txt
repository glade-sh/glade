capture: Datacloud.FindDuplicates verdicts and match confidences for custom matching rules
date: 2026-11-13
rules: Contact G4Last (LastName LastName), G4First (FirstName FirstName), G4Title (Title Title AND LastName Exact); Account G4Company (Name CompanyName), G4Street (BillingStreet Street), G4Three (Name CompanyName AND BillingCity City AND BillingStreet Street); all NullNotAllowed; duplicate rules Allow with alert
probe: FuzzyPanel.run inserts one record per value (DMLOptions allowSave), then calls findDuplicates on unsaved copies of every value; each match of the probed rule is candidate index, existing index (base 36) and the confidence when not 100; record ids are compared inside Apex and never exported
apis: 62.0 and 67.0; class ApiVersion read back after each deploy
routes: anonymous Apex and named @IsTest methods; two repeats each
log guard: each run fails on Apex log truncation, an incomplete observation frame or a class API mismatch; none occurred
ids: no record, org or user id, username or alias is stored

fit run (groups keyboard, title, street, company, first, multi; 13 cases)
org: a fresh 30-day Developer Edition scratch org on a campaign Dev Hub, deleted after the run; deletion verified (active scratch record 1 -> 0, local target no longer resolves)
result: all 104 observations agree across API, route and repeat
predictions: frozen before the native run from origin/main 0574a2848 (glade exec, API 67.0, GLADE_STANDARD_DUPLICATE_RULES=on); sha256 02a0a4cec64091ab27472e14d239f49fc58044053cbabdf46a4655bfa75502ca
before: the frozen predictions matched 758 of 1496 native pairs (API 67.0 @IsTest) with 4 false matches (J.R./J. R. both ways, the Fennimore and Callowmere plurals)

held-out run (groups hkeyboard, htitle, hstreet, hcompany, hfirst; 7 cases; values not used to fit)
org: a second fresh 30-day Developer Edition scratch org on another campaign Dev Hub, deleted after the run; deletion verified the same way
result: all 56 observations agree across API, route and repeat
predictions: frozen before the native run from the fitted model, commit 0c2615937; sha256 f14bc7ec5d7a540cec3240d9db4222408858b89ac71ba0e7e54c5087d9d661bc
scored once: 597 of 684 native pairs matched, 3 false matches: Wexmoore finds Wexmoores (an origin/main plural rule) and Thistlecombe/Thistlepowbe both ways (two far-key substitutions at keyboard score 90.67). Two rules were then changed: the "re" plural exclusion covers eight letters, and two substitutions keep the origin/main gate of 92.

cases.json sha256: 77c9e9877f2cb09885e4a5ef5858fe90cff179bf7f1deb03feda5dda852e138a
FuzzyPanel.cls sha256: 84c8cf5feb12044b138599f5268b9d2e453413741a4360fbaa66cb8b5d4d3ab2
org.tsv sha256: f64b99e20091cba84f21b425f149920a44fc8f9ad009ae7fb4b8a3284e485ce8
