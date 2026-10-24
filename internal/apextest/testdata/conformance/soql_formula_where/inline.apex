Savepoint marker=Database.setSavepoint();
List<Account> samples = new List<Account>{
 new Account(Name='Arithmetic A',AnnualRevenue=12.5,NumberOfEmployees=2,Start__c=Date.newInstance(2026,1,1),Finish__c=Date.newInstance(2026,1,4),StartTime__c=Datetime.newInstanceGmt(2026,1,1,0,0,0),FinishTime__c=Datetime.newInstanceGmt(2026,1,2,12,0,0)),
 new Account(Name='Arithmetic B',AnnualRevenue=3,NumberOfEmployees=5),
 new Account(Name='Arithmetic C',NumberOfEmployees=4),
 new Account(Name='Arithmetic D',AnnualRevenue=0)
};
insert samples;
List<Account> found=[SELECT Name FROM Account WHERE FORMULA('AnnualRevenue - NumberOfEmployees') > 5];
Database.rollback(marker);System.debug('OBS|'+found.size());
