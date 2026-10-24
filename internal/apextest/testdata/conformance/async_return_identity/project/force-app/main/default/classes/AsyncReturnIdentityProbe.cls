public class AsyncReturnIdentityProbe {
    public static Map<String,Object> observe(String operation) {
        Object returned;
        String assigned;
        if (operation == 'enqueue-default') { returned = System.enqueueJob(new ReturnIdentityQueue()); assigned = System.enqueueJob(new ReturnIdentityQueue()); }
        else if (operation == 'enqueue-integer') { returned = System.enqueueJob(new ReturnIdentityQueue(), 0); assigned = System.enqueueJob(new ReturnIdentityQueue(), 0); }
        else if (operation == 'enqueue-options') { returned = System.enqueueJob(new ReturnIdentityQueue(), new AsyncOptions()); assigned = System.enqueueJob(new ReturnIdentityQueue(), new AsyncOptions()); }
        else if (operation == 'batch-default') { returned = Database.executeBatch(new ReturnIdentityBatch()); assigned = Database.executeBatch(new ReturnIdentityBatch()); }
        else if (operation == 'batch-scope') { returned = Database.executeBatch(new ReturnIdentityBatch(), 1); assigned = Database.executeBatch(new ReturnIdentityBatch(), 1); }
        else { System.assert(false, 'Unknown operation'); return null; }
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
        return observation;
    }
    public static Map<String,Object> controls() {
        Id shortId = '707000000000001';
        Id longId = '707000000000001AAA';
        Object boxedShort = shortId;
        Object boxedLong = longId;
        Object stringShort = '707000000000001';
        Object stringLong = '707000000000001AAA';
        Map<String,Object> observation = new Map<String,Object>{
            'typedShortLength' => String.valueOf(shortId).length(),
            'typedLongLength' => String.valueOf(longId).length(),
            'boxedShortLength' => String.valueOf(boxedShort).length(),
            'boxedLongLength' => String.valueOf(boxedLong).length(),
            'castShortLength' => String.valueOf((Id) boxedShort).length(),
            'castLongLength' => String.valueOf((Id) boxedLong).length(),
            'stringShortLength' => String.valueOf(stringShort).length(),
            'stringLongLength' => String.valueOf(stringLong).length(),
            'nullValue' => String.valueOf((Object) null)
        };
        return observation;
    }
    public static String signature(Id value) { return 'Id'; }
    public static String signature(String value) { return 'String'; }
    public static Map<String,Object> directControls() {
        Map<String,Object> observation = new Map<String,Object>{
            'queueValueOfLength' => String.valueOf(System.enqueueJob(new ReturnIdentityQueue())).length(),
            'batchValueOfLength' => String.valueOf(Database.executeBatch(new ReturnIdentityBatch())).length(),
            'queueConcatLength' => ('' + System.enqueueJob(new ReturnIdentityQueue())).length(),
            'batchConcatLength' => ('' + Database.executeBatch(new ReturnIdentityBatch())).length(),
            'queueSignature' => AsyncReturnIdentityProbe.signature(System.enqueueJob(new ReturnIdentityQueue())),
            'batchSignature' => AsyncReturnIdentityProbe.signature(Database.executeBatch(new ReturnIdentityBatch())),
            'queueSObjectType' => String.valueOf(System.enqueueJob(new ReturnIdentityQueue()).getSObjectType())
        };
        return observation;
    }
    public static Map<String,Object> memberControls() {
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
        return observation;
    }
    public static Integer stringLength(String value) { return value.length(); }
    public static String stringReturn() { return System.enqueueJob(new ReturnIdentityQueue()); }
    public static Id idReturn() { return System.enqueueJob(new ReturnIdentityQueue()); }
    public static Object objectReturn() { return System.enqueueJob(new ReturnIdentityQueue()); }
    public static Map<String,Object> typedReturnControls() {
        Id submitted = System.enqueueJob(new ReturnIdentityQueue());
        Object boxed = submitted;
        String assigned = submitted;
        String castText = (String) submitted;
        Map<String,Object> observation = new Map<String,Object>{
            'typedValueOfLength' => String.valueOf(submitted).length(),
            'boxedValueOfLength' => String.valueOf(boxed).length(),
            'typedConcatLength' => ('' + submitted).length(),
    'castIdValueOfLength' => String.valueOf((Id) submitted).length(),
    'castIdConcatLength' => ('' + (Id) submitted).length(),
            'typedAssignedLength' => assigned.length(),
            'typedCastLength' => castText.length(),
            'typedJsonLength' => JSON.serialize(submitted).length(),
            'stringArgumentLength' => AsyncReturnIdentityProbe.stringLength(System.enqueueJob(new ReturnIdentityQueue())),
            'stringReturnLength' => AsyncReturnIdentityProbe.stringReturn().length(),
            'idReturnValueOfLength' => String.valueOf(AsyncReturnIdentityProbe.idReturn()).length(),
            'objectReturnValueOfLength' => String.valueOf(AsyncReturnIdentityProbe.objectReturn()).length()
        };
        return observation;
    }
    public static Map<String,Object> objectEqualityControls() {
        String shortText = '707000000000001';
        String longText = '707000000000001AAA';
        Object shortObject = shortText;
        Object longObject = longText;
        Object ordinaryShort = 'abcdefghijklmno';
        Object ordinaryLong = 'abcdefghijklmnoAAA';
        Map<String,Object> observation = new Map<String,Object>{
            'objectJobEquals' => shortObject.equals(longObject),
            'objectJobReverseEquals' => longObject.equals(shortObject),
            'stringJobEquals' => shortText.equals(longText),
            'objectOrdinaryEquals' => ordinaryShort.equals(ordinaryLong),
            'objectMismatchEquals' => shortObject.equals('707000000000002AAA'),
            'objectSameLengthEquals' => ordinaryShort.equals('ABCDEFGHIJKLMNO')
        };
        return observation;
    }
}
