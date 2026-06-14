# LWC Shell Fixture

This SFDX project exercises local LWC shell discovery.

- `contextProbe` checks public `@api` attributes, labels, and static resources.
- `recordProbe` checks `lightning/uiRecordApi` record wiring.
- `wireProbe` checks Apex wire and imperative Apex imports.
- `Account_Record_Page`, `Sales_Dashboard`, `Custom_Home`, and `Lwc_Probe` cover record, app, home, and tab shells.

Run `sh validate_fixture.sh` from the repository root to check the fixture shape.
