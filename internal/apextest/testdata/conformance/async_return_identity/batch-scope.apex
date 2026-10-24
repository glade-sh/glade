Object returned = Database.executeBatch(new ReturnIdentityBatch(), 1);
String assigned = Database.executeBatch(new ReturnIdentityBatch(), 1);
Id converted = (Id) returned;
String rawText = String.valueOf(returned);
String idText = converted;
AsyncApexJob job = [SELECT Id, JobType FROM AsyncApexJob WHERE Id = :converted];
String queriedText = job.Id;
Map<String,Object> observation = new Map<String,Object>{
    'isId' => returned instanceof Id,
    'isString' => returned instanceof String,
    'rawLength' => rawText.length(),
    'assignedLength' => assigned.length(),
    'convertedLength' => idText.length(),
    'sObjectType' => String.valueOf(converted.getSObjectType()),
    'queriedLength' => queriedText.length(),
    'queriedSObjectType' => String.valueOf(job.Id.getSObjectType()),
    'idEqualsQueried' => converted == job.Id,
    'rawEqualsQueried' => rawText.equals(queriedText),
    'convertedEqualsQueried' => idText.equals(queriedText),
    'jobType' => job.JobType
};
System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(observation));
