Integer rolesBefore=[SELECT COUNT() FROM UserRole];
Savepoint point=Database.setSavepoint();
Map<String,Object> observation=RoleBoundaryProbe.observe(false);
Database.rollback(point);
System.assertEquals(rolesBefore, [SELECT COUNT() FROM UserRole]);
insert new Account(Name='Cleanup');
System.debug('OBS|' + JSON.serialize(observation));
