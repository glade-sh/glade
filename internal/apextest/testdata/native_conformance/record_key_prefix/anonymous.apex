List<Organization> rows = [SELECT Id FROM Organization];
System.assertEquals(1, rows.size());
System.assertEquals(UserInfo.getOrganizationId(), rows[0].Id);
Account acct = new Account(Name = 'probe');
Savepoint sp = Database.setSavepoint();
insert acct;
Boolean ok = String.valueOf(acct.Id).left(3) == Account.SObjectType.getDescribe().getKeyPrefix() && acct.Id.getSObjectType() == Account.SObjectType;
Database.rollback(sp);
System.assert(ok);
