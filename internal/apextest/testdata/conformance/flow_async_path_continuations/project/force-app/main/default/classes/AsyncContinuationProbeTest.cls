@IsTest private class AsyncContinuationProbeTest {
 static Map<String,Object> observe() { return new Map<String,Object>{'delayed'=>[SELECT count() FROM AsyncParcel__c WHERE Delayed__c=true],'trigger'=>[SELECT count() FROM Account WHERE Name='Async trigger continuation'],'flow'=>[SELECT count() FROM Account WHERE Name='Async flow continuation']}; }
 @IsTest static void EXECUTE() { Test.startTest();insert new AsyncParcel__c(Name='async',Source__c='initial');Test.stopTest();System.assert(false,'CONTINUATION|EXECUTE|'+JSON.serialize(observe())); }
 @IsTest static void ROLLBACK() {AsyncParcel__c row=new AsyncParcel__c(Name='other');insert row;Test.startTest();Savepoint point=Database.setSavepoint();row.Name='update';update row;Database.rollback(point);Test.stopTest();System.assert(false,'CONTINUATION|ROLLBACK|'+JSON.serialize(observe()));}
}
