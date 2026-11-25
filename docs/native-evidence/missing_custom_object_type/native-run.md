A Developer scratch org was used for anonymous Apex at APIs 62.0, 67.0 and 68.0. An absent custom object type was rejected both directly and inside a list. An existing standard object control compiled and ran. The org was deleted and deletion verified.

A separate fresh scratch org received versioned copies of an `@IsTest` class at the same APIs. Salesforce rejected each copy with the recorded first diagnostic. The capture wrapper then retried an empty deployment and exited with an infrastructure error. The rejected classes were never deployed, so their API versions are those in the generated deployment metadata, not values queried from the org. That org was also deleted and deletion verified.

Both scratch orgs were created with a 30-day duration. Rows are in `internal/apextest/testdata/conformance/missing_custom_object_type/org.tsv`; the named check-only rows at these APIs record the deployment rejections, not successful test executions.

A third fresh 30-day org rejected the named class at deployment metadata APIs 63.0 through 66.0 with the same first diagnostic. As above, the wrapper retried an empty deployment and exited with an infrastructure error; no class API was queried. A fourth fresh 30-day org contained a deployed synthetic custom object. Anonymous Apex and a named test using it passed at APIs 62.0, 67.0 and 68.0. Both orgs were deleted and deletion verified.

The replay checks that semantic analysis and the test runner reject the named missing-object class with the recorded deployment diagnostic.
