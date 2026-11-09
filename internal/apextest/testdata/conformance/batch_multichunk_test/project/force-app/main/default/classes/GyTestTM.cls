@IsTest private class GyTestTM {
 @IsTest static void qlMulti(){insert new List<Account>{new Account(Name='zz-in'),new Account(Name='zz-in')};Test.startTest();Database.executeBatch(new GyBatchQL(),1);Test.stopTest();Integer n=[SELECT COUNT() FROM Account WHERE Name='chunk'];System.assert(false,'CAP|qlMulti|reached|chunks='+n);}
 @IsTest static void qlSingle(){insert new List<Account>{new Account(Name='zz-in'),new Account(Name='zz-in')};Test.startTest();Database.executeBatch(new GyBatchQL(),2);Test.stopTest();Integer n=[SELECT COUNT() FROM Account WHERE Name='chunk'];System.assert(false,'CAP|qlSingle|reached|chunks='+n);}
 @IsTest static void itMulti(){insert new List<Account>{new Account(Name='zz-in'),new Account(Name='zz-in')};Test.startTest();Database.executeBatch(new GyBatchIt(),1);Test.stopTest();Integer n=[SELECT COUNT() FROM Account WHERE Name='chunk'];System.assert(false,'CAP|itMulti|reached|chunks='+n);}
}
