@IsTest private class GyTestSetupQlSingle {
 @TestSetup static void s(){insert new List<Account>{new Account(Name='zz-in'),new Account(Name='zz-in')};Test.startTest();Database.executeBatch(new GyBatchQL(),2);Test.stopTest();}
 @IsTest static void t(){Integer n=[SELECT COUNT() FROM Account WHERE Name='chunk'];System.assert(false,'CAP|setupQlSingle|reached|chunks='+n);}
}
