Map<String,Object> out = new Map<String,Object>();
Schema.DescribeSObjectResult d = User.SObjectType.getDescribe();
out.put('userCreate',d.isCreateable()); out.put('userRead',d.isAccessible()); out.put('userUpdate',d.isUpdateable()); out.put('userDelete',d.isDeletable());
Profile p = [SELECT PermissionsModifyAllData, PermissionsViewAllData, PermissionsManageUsers FROM Profile WHERE Id = :UserInfo.getProfileId()];
out.put('modifyAllData',p.PermissionsModifyAllData); out.put('viewAllData',p.PermissionsViewAllData); out.put('manageUsers',p.PermissionsManageUsers);
User u = new User(LastName='Synthetic', Alias='probe', Email='probe@example.invalid', Username='probe.'+String.valueOf(Crypto.getRandomLong())+'@example.invalid', ProfileId=UserInfo.getProfileId(), LocaleSidKey='en_US', LanguageLocaleKey='en_US', EmailEncodingKey='UTF-8', TimeZoneSidKey='America/Los_Angeles');
Database.UpsertResult first = Database.upsert(u, User.Id, false); out.put('firstSuccess',first.isSuccess()); out.put('firstCreated',first.isCreated()); if (!first.isSuccess()) out.put('firstError',String.valueOf(first.getErrors()[0].getStatusCode()));
u.LastName='Changed'; Database.UpsertResult second = Database.upsert(u, User.Id, false); out.put('secondSuccess',second.isSuccess()); out.put('secondCreated',second.isCreated()); out.put('secondErrors',second.getErrors().size()); if (!second.isSuccess()) out.put('secondError',String.valueOf(second.getErrors()[0].getStatusCode()));
System.debug(LoggingLevel.ERROR,'CAP|'+JSON.serialize(out));
