Fresh 30-day Developer scratch org; org created for this capture and deleted after use.
API 62.0 and 67.0: each version's TransformProbe metadata deployed successfully and its anonymous probe returned the corresponding row.
Both test classes passed against the API 62.0 TransformProbe deployment; each class used its own indicated Apex API version.
TransformReject metadata was deployed separately at API 62.0 and API 67.0. Both attempts failed with a field integrity exception naming the unknown target field.
