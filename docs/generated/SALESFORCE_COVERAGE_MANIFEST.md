# Salesforce Coverage Manifest

- Source documents: 15635
- Source members: 6768
- Coverage entries: 22403
- Known supported entries: 168
- Unknown entries: 7501
- Tooling API classes: 7091
- Tooling API members: 73326
- Runtime APIs found in Tooling API: 133/134

| Area | Target | Entries | Supported | Partial | Stub | Unsupported | Unknown |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Core stdlib | `executable-parity` | 1080 | 102 | 32 | 0 | 0 | 946 |
| Data platform | `local-model` | 964 | 59 | 13 | 0 | 0 | 892 |
| Integration, security, and UI | `local-model` | 766 | 6 | 20 | 0 | 0 | 740 |
| Language and guide docs | `unsupported` | 14662 | 0 | 0 | 0 | 14662 | 0 |
| Product namespaces | `typed-stub` | 4782 | 0 | 0 | 0 | 0 | 4782 |
| Tests, async, and limits | `local-model` | 149 | 1 | 7 | 0 | 0 | 141 |

## Tooling API System Alignment

Source: `tooling_system_symbols.json.gz`

- Namespaces: 145
- Classes: 7091
- Constructors: 5807
- Methods: 40522
- Properties: 26997
- System-default namespace classes: 198
- System-default namespace members: 3280
- Concrete runtime APIs in Tooling API: 133/134
- Catalog system entries in Tooling API: 2136/2959

### Runtime APIs Not Found In Tooling API
- `Time.valueOf`

### Catalog System Entries Not Found In Tooling API
- `AccessLevel.SYSTEM\_MODE`
- `AccessLevel.USER\_MODE`
- `AccessLevel.withPermissionSetId`
- `AggregateResult`
- `ApexPages.ApexPages`
- `ApexPages.KnowledgeArticleVersionStandardController.setDataCategory`
- `ApexPages.StandardController.reset`
- `Approval.Approval`
- `Approval.process`
- `Approval.process`
- `Approval.process`
- `Approval.process`
- `Auth.Auth`
- `Auth.AuthConfiguration`
- `Auth.AuthConfiguration.AuthConfiguration`
- `Auth.AuthConfiguration.getAllowInternalUserLoginEnabled`
- `Auth.AuthConfiguration.getAuthConfig`
- `Auth.AuthConfiguration.getAuthConfigProviders`
- `Auth.AuthConfiguration.getAuthProviders`
- `Auth.AuthConfiguration.getAuthProviderSsoDomainUrl`
- `Auth.AuthConfiguration.getAuthProviderSsoUrl`
- `Auth.AuthConfiguration.getBackgroundColor`
- `Auth.AuthConfiguration.getCertificateLoginEnabled`
- `Auth.AuthConfiguration.getCertificateLoginUrl`
- `Auth.AuthConfiguration.getDefaultProfileForRegistration`
