Map<String,Object> observation = new Map<String,Object>{
    'queueValueOfLength' => String.valueOf(System.enqueueJob(new ReturnIdentityQueue())).length(),
    'batchValueOfLength' => String.valueOf(Database.executeBatch(new ReturnIdentityBatch())).length(),
    'queueConcatLength' => ('' + System.enqueueJob(new ReturnIdentityQueue())).length(),
    'batchConcatLength' => ('' + Database.executeBatch(new ReturnIdentityBatch())).length(),
    'queueSignature' => AsyncReturnIdentityProbe.signature(System.enqueueJob(new ReturnIdentityQueue())),
    'batchSignature' => AsyncReturnIdentityProbe.signature(Database.executeBatch(new ReturnIdentityBatch())),
    'queueSObjectType' => String.valueOf(System.enqueueJob(new ReturnIdentityQueue()).getSObjectType())
};
System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(observation));
