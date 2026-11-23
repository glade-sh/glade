# Native capture: $Profile.Name and RecordType.DeveloperName decisions in an after-save update Flow

An Account after-save update flow decides `$Profile.Name NotEqualTo 'Nobody Profile'` AND `$Record.RecordType.DeveloperName NotEqualTo` a String constant, then `RecordType.DeveloperName EqualTo` a literal. An Admin profile makes both record types visible, with Other_Type as the default. Salesforce answered `n,y,Probe_Type|m,n,Other_Type|m,n,Other_Type`.

Fixture: `internal/apextest/testdata/conformance/flow_record_type_decision` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows cover Apex class API 62.0 and 67.0, anonymous and @IsTest routes. The flows were deployed at 67.0 in both orgs. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version, and Salesforce ids are masked. Every org was deleted and verified gone after the run.
