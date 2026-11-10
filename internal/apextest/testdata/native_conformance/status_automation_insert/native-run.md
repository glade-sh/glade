# Native run

A fresh 30-day Developer scratch org was used on 2026-11-09. Salesforce CLI 2.156.8 deployed the object and field metadata, assigned the synthetic permission set, and ran anonymous Apex at API 62.0 and 67.0 before any flows were installed. The active flows were then deployed at API 62.0 and the case ran at 62.0; the flows were redeployed at API 67.0 and the case ran at 67.0. The org was deleted; its Dev Hub record showed Deleted and no active matching scratch org.

`org.tsv` retains the captured rows, including positive and negative controls. Values in `exact_pre` are status, handler status and whether the related request lookup survived field-access filtering. Values in `exact_insert` are status, handler status, object marker, request marker, requested flag, email-sent flag, notification marker and message count. All rows are anonymous execution observations.

The saved metadata is the API 67.0 variant; only its flow `apiVersion` was changed for the 62.0 deployment. The saved `captured-cases.apex` contains all deployed statements with the diagnostic prefix normalized; it is not byte-for-byte identical to the deployed source. The saved `anonymous.apex` is its exact-insert prefix, with subsequent positive-control statements removed. Neither change modifies the measured DML or values. `org.tsv` includes the original positive-control rows; this narrow replay checks only exact_pre and exact_insert. Separate confirmation and external-message positive controls did not match local execution and are not tested by this replay.

This is a synthetic isolation model. It uses an unrelated marker for an account-field update and a record type developer name for a literal ID. Notification and message actions are simplified, and dependent automation and helper DML are not modeled. These observations do not prove the behavior of a complete application with additional automation.

Commands, with the scratch target and local source paths omitted:

```sh
sf project deploy start --source-dir force-app --target-org <scratch-org> --wait 10 --json
sf org assign permset --name ProbeAccess --target-org <scratch-org> --json
sf apex run --target-org <scratch-org> --api-version 62.0 --file cases.apex --json
sf apex run --target-org <scratch-org> --api-version 67.0 --file cases.apex --json
sf project deploy start --source-dir force-app/main/default/flows --target-org <scratch-org> --wait 10 --json
sf apex run --target-org <scratch-org> --api-version 62.0 --file cases.apex --json
# Change only each flow's apiVersion to 67.0.
sf project deploy start --source-dir force-app/main/default/flows --target-org <scratch-org> --wait 10 --json
sf apex run --target-org <scratch-org> --api-version 67.0 --file cases.apex --json
sf org delete scratch --target-org <scratch-org> --no-prompt --json
```
