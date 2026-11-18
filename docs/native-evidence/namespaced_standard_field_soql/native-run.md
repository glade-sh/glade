# Native run: namespaced custom field on a standard object, package installed

Question: Glade's SOQL engine treats any name shaped `ns__Field__c` as a known
field of a standard object (soql.go namespacedCustomFieldLikeSOQLName). Does
Salesforce accept such a name, and does the answer depend on the package?

Namespace prefixes are masked: the captured packages used the namespaces of the
development orgs, written here as `pkgone` (the installed package) and `pkgnine`
(a namespace that exists nowhere). Messages in org.tsv carry the masked names.

Orgs (fresh 30-day Developer-edition scratch orgs; no ids or usernames kept):
- `package-installed`: the beta version of a managed package (namespace
  pkgone) is installed. The package ships Contact.pkgone__ParentRouteValue__c
  and a custom object. The org also has the StateAndCountryPicklist feature.
- `plain-no-package`: same edition, default settings, no package.

Source API 62.0 and 67.0, anonymous route (Tooling executeAnonymous; a
statement that compiles ends in a thrown `ok`). Each case ran twice per API.
Cases are in cases.json: static, WHERE, ORDER BY, dynamic and parent-path
queries on Contact and Account, plus `describe`, `put` and `get` controls.

Result:
- Installed org: `pkgone__ParentRouteValue__c` is accepted in SELECT, WHERE
  and `Database.query`, and the describe map lists it.
- Installed org: `pkgone__NoSuchField__c` (the package's namespace, absent
  field), `pkgnine__ParentRouteValue__c` (other namespace), the bare
  `ParentRouteValue__c`, `Foo__Bar__c` and a parent path such as
  `Account.pkgone__NoSuchField__c` are all rejected: `No such column ...`
  at compile time for static queries, `System.QueryException` for dynamic ones.
  `put` of the absent name throws `SObjectException: Invalid field ...`.
- Plain org: every one of those names is rejected, including the package field.
- Standard and custom object targets behave alike (Contact and Account).

So the tolerance is correct only when the named package is installed and has the
field. Glade cannot tell: its runtime state lists no installed packages, and a
declared dependency without source gives only a namespace.

Parked: narrowing the tolerance needs the project's declared package
dependencies (the namespace binding work), so this change records the rows and
pins the shapes Glade already matches. The sema static check has a matching
tolerance (allowsIncompleteExternalManagedPackageField).
