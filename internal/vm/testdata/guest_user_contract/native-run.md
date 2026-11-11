# guest_user_contract native run

Route: @IsTest (`sf apex run test`, synchronous), one class per API version
(`GuestUserContract` at 67.0, `GuestUserContract62` at 62.0). Each method fails
on purpose so the observation travels in the assertion message; `org.tsv` holds
that message.

Org: 1-day Developer scratch org with the Communities feature, one deployed
`CustomSite` (this creates a Guest-type profile). The guest principal is a new
`User` on the Guest-type profile, run through `System.runAs`; the control is a
new `User` on `Standard User`. No org id, user id or username is stored.

Observed on 2026-11-10 (UTC), `sf` CLI 2.x, the org deleted afterwards.

`describeFlags`: `obj` is `isAccessible,isCreateable,isUpdateable,isDeletable,isQueryable`
for `Schema.SObjectType.User` as the administrator, then `/` the same five as the guest.
`diffs` lists User fields whose accessible/createable/updateable flags differ between
the guest and the Standard User (`numberoffailedlogins` excluded: see `soqlRows`).

`soqlRows`: an inserted guest user queries its own `User` row one column at a time,
then `SELECT Id FROM User` and an `IsActive` filter.
