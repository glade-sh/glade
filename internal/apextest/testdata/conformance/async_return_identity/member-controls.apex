Object queueReturned = System.enqueueJob(new ReturnIdentityQueue());
Id queueId = (Id) queueReturned;
AsyncApexJob queueJob = [SELECT Id FROM AsyncApexJob WHERE Id = :queueId];
String queueQueriedText = queueJob.Id;
Object batchReturned = Database.executeBatch(new ReturnIdentityBatch());
Id batchId = (Id) batchReturned;
AsyncApexJob batchJob = [SELECT Id FROM AsyncApexJob WHERE Id = :batchId];
String batchQueriedText = batchJob.Id;
String queueDirectCast = (String) System.enqueueJob(new ReturnIdentityQueue());
String batchDirectCast = (String) Database.executeBatch(new ReturnIdentityBatch());
String queueObjectCast = (String) queueReturned;
String batchObjectCast = (String) batchReturned;
Map<String,Object> observation = new Map<String,Object>{
    'queueDirectCastLength' => queueDirectCast.length(),
    'batchDirectCastLength' => batchDirectCast.length(),
    'queueObjectCastLength' => queueObjectCast.length(),
    'batchObjectCastLength' => batchObjectCast.length(),
    'queueObjectEqualsId' => queueReturned.equals(queueJob.Id),
    'queueObjectEqualsString' => queueReturned.equals(queueQueriedText),
    'batchObjectEqualsId' => batchReturned.equals(batchJob.Id),
    'batchObjectEqualsString' => batchReturned.equals(batchQueriedText),
    'queueTo15Length' => System.enqueueJob(new ReturnIdentityQueue()).to15().length(),
    'queueEqualsNull' => System.enqueueJob(new ReturnIdentityQueue()).equals(null),
    'queueDirectJsonLength' => JSON.serialize(System.enqueueJob(new ReturnIdentityQueue())).length(),
    'batchDirectJsonLength' => JSON.serialize(Database.executeBatch(new ReturnIdentityBatch())).length(),
    'queueObjectJsonLength' => JSON.serialize(queueReturned).length(),
    'batchObjectJsonLength' => JSON.serialize(batchReturned).length()
};
System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(observation));
