public with sharing class RecordAccessOperationProbe {
    public static Map<String,Object> observe(String kind,SObject record) {
        Map<String,Object> row = new Map<String,Object>();
        try {
            if (kind == 'userUpsert') Database.upsert(new List<SObject>{record,new Case(Subject='Fresh',Status='New',Origin='Web')},true,AccessLevel.USER_MODE);
            if (kind == 'userUndelete') Database.undelete(new List<SObject>{record},true,AccessLevel.USER_MODE);
            if (kind == 'systemUpsert') Database.upsert(new List<SObject>{record,new Contact(LastName='Fresh')},true,AccessLevel.SYSTEM_MODE);
            if (kind == 'systemUndelete') Database.undelete(new List<SObject>{record},true,AccessLevel.SYSTEM_MODE);
            if (kind == 'systemUpdate') {
                Database.DMLOptions options = new Database.DMLOptions();
                options.optAllOrNone = true;
                Database.update(new List<SObject>{record},options,AccessLevel.SYSTEM_MODE);
            }
            row.put('success',true);
        } catch (Exception error) {
            row.put('success',false);
            row.put('exceptionType',error.getTypeName());
            String message = error.getMessage();
            String recordId = String.valueOf(record.Id);
            message = message.replace(recordId,'RECORD').replace(recordId.substring(0,15),'RECORD');
            row.put('message',message);
        }
        return row;
    }
}
