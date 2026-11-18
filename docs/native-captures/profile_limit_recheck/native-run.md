# Profile ordered LIMIT recapture

Captured 2026-11-10 in three fresh 30-day Developer scratch orgs: a plain org, an org created with Communities enabled, and a plain org with the Communities setting deployed afterward. All three were created for this probe and deleted afterward; the Dev Hub reported each scratch-org status as Deleted. The captures contain no org IDs, usernames, aliases, or scratch hostnames.

The minimal synthetic project contains `ProfileProbe.cls` and `ProfileProbeTest.cls`. `configurations/plain.json` creates the plain org; `configurations/communities_feature.json` creates the feature-at-creation org. For the settings-only org, create with `configurations/plain.json` and deploy `variants/settings-only/Communities.settings-meta.xml` into the project alongside the classes. A Developer scratch org and synthetic project files suffice; no existing org or corpus metadata is used.

For each source API version 62.0 and 67.0, set both class metadata `<apiVersion>` values to that version, then run the following commands from the project directory with a freshly created scratch org selected as `SCRATCH_ORG`:

```sh
sf project deploy start --source-dir force-app --target-org "$SCRATCH_ORG" --api-version "$API_VERSION" --wait 10 --json
sf apex run --target-org "$SCRATCH_ORG" --file ../observe.apex --api-version "$API_VERSION" --json
sf apex run test --class-names ProfileProbeTest --target-org "$SCRATCH_ORG" --synchronous --result-format json --api-version "$API_VERSION" --wait 10 --json
```

The anonymous and `@IsTest` commands were each repeated three times for each version and configuration. The test deliberately transports its observation in an assertion message. The query selects Profile Name and UserLicense.Name with `ORDER BY Name LIMIT 1`. `org.tsv` retains each of the 36 observations. The earlier `standard_profile_inventory/org.tsv` contains full-inventory and missing-record controls, and `standard_profile_inventory/limit-ordering-recheck.tsv` retains the earlier same-org redeployment observations. No native rows have been altered.

All 36 new observations selected the same inventory member. The earlier `standard_profile_inventory/org.tsv` records a different member for ten corresponding configuration/API/route cells. Its remaining two cells agree with the new observations but differ from the earlier same-org redeployment recheck. All twelve cells are therefore variable under the captured observations.
