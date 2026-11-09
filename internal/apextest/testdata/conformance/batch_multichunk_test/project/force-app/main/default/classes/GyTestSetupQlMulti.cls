@IsTest private class GyTestSetupQlMulti {
 @TestSetup static void s(){insert new List<Account>{new Account(Name='zz-in'),new Account(Name='zz-in')};Test.startTest();Database.executeBatch(new GyBatchQL(),1);Test.stopTest();}
 @IsTest static void t(){Integer n=[SELECT COUNT() FROM Account WHERE Name='chunk'];System.assert(false,'CAP|setupQlMulti|reached|chunks='+n);}
}
