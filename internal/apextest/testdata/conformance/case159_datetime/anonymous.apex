Savepoint sp=Database.setSavepoint();
Datetime a=Datetime.newInstance(1777723200007L);
RoadTime__c row=new RoadTime__c(Name='Synthetic',Moment__c=a); insert row;
RoadTime__c back=[SELECT Moment__c FROM RoadTime__c WHERE Id=:row.Id];
System.debug('CASE159|insert|'+a.getTime()+'|'+row.Moment__c.getTime()+'|'+back.Moment__c.getTime());
row.Moment__c=Datetime.newInstance(1777723200999L); update row;
back=[SELECT Moment__c FROM RoadTime__c WHERE Id=:row.Id];
System.debug('CASE159|update|'+row.Moment__c.getTime()+'|'+back.Moment__c.getTime());
Database.rollback(sp);
