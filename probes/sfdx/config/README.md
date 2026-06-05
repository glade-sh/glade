# Scratch Org Probe Configs

`project-scratch-def.json` is the broad baseline probe org.

`project-scratch-def.objectsfields-max.json` is a best-effort Data.Reference.ObjectsFields probe org. It keeps the baseline shape and adds feature families that can expose more standard objects in describe results: event monitoring and streams, field history, scheduler, field service, analytics, platform events, chatbot, assessments, big objects, and platform connect.

Create it from this repo root with a Dev Hub that has the matching entitlements:

```bash
sf org create scratch \
  --target-dev-hub <dev-hub-alias> \
  --definition-file probes/sfdx/config/project-scratch-def.objectsfields-max.json \
  --alias oaer-objectsfields-max \
  --duration-days 7 \
  --wait 30
```

This file avoids the heaviest entitlement clusters that can pass definition validation and still fail signup with `RemoteOrgSignupFailed`: commerce, loyalty, public sector, financial services, industries, sustainability, CPQ, billing, document generation, OmniStudio, and service voice. Probe those in smaller org definitions if the Dev Hub has matching licenses. The remaining missing ObjectsFields rows should be measured from the resulting org describe output, not assumed from the config alone.

Salesforce scratch definition values are documented at:
https://developer.salesforce.com/docs/atlas.en-us.sfdx_dev.meta/sfdx_dev/sfdx_dev_scratch_orgs_def_file_config_values.htm
