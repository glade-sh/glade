List<PermissionSet> sets = [SELECT Profile.Id, Profile.Name FROM PermissionSet WHERE IsOwnedByProfile = true AND Profile.UserType = 'Standard' AND Id NOT IN (SELECT ParentId FROM ObjectPermissions WHERE SObjectType = 'Account' AND PermissionsRead = true)];
List<String> names = new List<String>(); for (PermissionSet ps : sets) names.add(ps.Profile.Name); names.sort();
System.debug('OBS|' + JSON.serialize(new Map<String,Object>{ 'profileCount' => names.size(), 'profileNames' => names }));
