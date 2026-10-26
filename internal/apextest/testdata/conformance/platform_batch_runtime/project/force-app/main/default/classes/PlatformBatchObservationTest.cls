@IsTest private class PlatformBatchObservationTest {
 @IsTest static void directHooks() { System.assert(false,'OBSERVED|directHooks|' + JSON.serialize(PlatformBatchObservation.directHooks())); }
 @IsTest static void inheritedStart() { System.assert(false,'OBSERVED|inheritedStart|' + JSON.serialize(PlatformBatchObservation.inheritedStart())); }
 @IsTest static void inheritedExecute() { System.assert(false,'OBSERVED|inheritedExecute|' + JSON.serialize(PlatformBatchObservation.inheritedExecute())); }
 @IsTest static void batchStop() {
 Map<String,Object> observed = new Map<String,Object>();
 try {
  Test.startTest();
  Id job = Database.executeBatch(PlatformBatchObservation.detached());
  Test.stopTest();
  AsyncApexJob result = [SELECT Status,NumberOfErrors FROM AsyncApexJob WHERE Id=:job];
  observed.put('status',result.Status); observed.put('errors',result.NumberOfErrors);
 } catch (Exception error) { observed.put('errorType',error.getTypeName()); observed.put('errorMessage',error.getMessage()); }
 observed.put('finished',[SELECT count() FROM Account WHERE Name='Platform batch finish']);
 System.assert(false,'OBSERVED|batchStop|' + JSON.serialize(observed));
 }
 @IsTest static void emptyConstructor() { System.assert(false,'OBSERVED|emptyConstructor|' + JSON.serialize(PlatformBatchObservation.emptyConstructor())); }
}
