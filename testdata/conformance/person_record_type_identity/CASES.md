# Record type identity boundaries

Owned synthetic metadata defines three Account/PersonAccount pairs:
Business_Care / Care, Business_Member / Member, Business_Provider / Provider.
Each business label equals the corresponding person developer name; the person
label adds ` Label`. The identities must remain distinct.

| Cases | Native route | Required observations |
| --- | --- | --- |
| Care, Member, Provider identity | anonymous / @IsTest, 62/67 | Different IDs; both labels preserved |
| Business inserts | anonymous / @IsTest, 62/67 | Success, selected ID preserved, IsPersonAccount false |
| Person inserts | anonymous / @IsTest, 62/67 | Success, selected ID preserved, IsPersonAccount true |
| Missing LastName | anonymous / @IsTest, 62/67 | REQUIRED_FIELD_MISSING |
| Label reference | Profile / PermissionSet deploy, 62/67 | Rejected: no matching developer identity |
| Absent qualified reference | Profile / PermissionSet deploy, 62/67 | Rejected despite existing unqualified Care |

The offline tests exercise these rows through project, anonymous, cold cache,
warm cache and restored disk cache routes. Additional owned metadata tests cover
folding, profile loading, permission loading and profile-owned permission loading.
Qualified identities in those tests are structural isolation controls: this
capture does not install a namespaced package or establish namespace aliasing.
Promotion is narrowed to exact full developer identity; labels and namespace
suffixes do not authorize promotion of another type. Source-reference discovery
retains its existing label/namespace existence matcher.
