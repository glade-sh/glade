Integer rolesBefore=[SELECT COUNT() FROM UserRole];
Savepoint point=Database.setSavepoint();
Map<String,Object> observation=RoleBoundaryProbe.observe(true);
Database.rollback(point);
System.assertEquals(rolesBefore, [SELECT COUNT() FROM UserRole]);
insert new PermissionSet(Name='Cleanup' + String.valueOf(DateTime.now().getTime()), Label='Cleanup');
System.debug('OBS|' + JSON.serialize(observation));
