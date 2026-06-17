# LWC Shell Fixture

This SFDX project exercises local LWC shell discovery and context selection.

- `contextProbe` checks public `@api` attributes, labels, and static resources.
- `recordProbe` checks `lightning/uiRecordApi` record wiring.
- `wireProbe` checks Apex wire and imperative Apex imports.
- `actionProbe` checks URL-addressable and quick action contexts.
- `layoutProbe` checks `lightning/uiLayoutApi` `getLayout` wiring.
- `objectInfoProbe` checks `lightning/uiObjectInfoApi` object info and picklist wiring.
- `relatedListProbe` checks `lightning/uiRelatedListApi` child-row wiring.
- `Account_Record_Page`, `Sales_Dashboard`, `Custom_Home`, and `Lwc_Probe` cover record, app, home, and tab shells.
- `Lwc_Shell` and `Lwc_Shell_Access` make the custom-tab browser oracle visible in a scratch org.
- `Account.Update_Status` and `Global_Status` cover LWC-backed quick action shells.
- `glade.lwc.json` defines route contexts plus direct service contexts:
  `accountRecord`, `salesDashboard`, `home`, `tab`,
  `urlAddressableAction`, `recordAction`, `globalAction`, `apexWire`,
  `ldsRecord`, `uiObjectInfo`, `uiRelatedList`, `uiLayout`, and
  `baseComponents`.

Use it from the repository root:

```bash
glade dev lwc --project testdata/local-tests/lwc-shell --context accountRecord --open
glade dev lwc --project testdata/local-tests/lwc-shell --context ldsRecord --open
curl http://127.0.0.1:8080/lightning/local/context.json
```

Run `sh validate_fixture.sh` from the repository root to check the fixture shape.
