public without sharing class RecordOperationProbe {
    public static Map<String,Object> observe(String kind) {
        Map<String,Object> row = new Map<String,Object>();
        try {
            if (kind == 'identifierAccount' || kind == 'identifierCustom') {
                SObject record = kind == 'identifierAccount' ? (SObject)new Account(Name='Before') : (SObject)new ProbeNote__c(Name='Before');
                insert record;
                String idText = String.valueOf(record.Id);
                Boolean zeroSegment = idText.substring(3,9) == '000000';
                row.put('zeroSegment', zeroSegment);
                row.put('length', idText.length());
                if (!zeroSegment) {
                    record.put('Name', 'After');
                    update record;
                }
                String objectName = kind == 'identifierAccount' ? 'Account' : 'ProbeNote__c';
                Id recordId = record.Id;
                SObject saved = Database.query('SELECT Name FROM '+objectName+' WHERE Id = :recordId');
                row.put('updated', saved.get('Name') == 'After');
            } else {
                Account master = new Account(Name='Primary');
                Account first = new Account(Name='First');
                Account second = new Account(Name='Second');
                insert new List<Account>{master,first,second};
                List<Account> duplicates = new List<Account>{first,second};
                if (kind == 'mergePartial') delete second;
                if (kind == 'mergeEmpty') duplicates.clear();
                Database.MergeResult[] results = Database.merge(master,duplicates,false);
                List<Object> details = new List<Object>();
                for (Database.MergeResult result : results) {
                    List<String> errors = new List<String>();
                    List<Database.Error> resultErrors = result.getErrors();
                    if (resultErrors != null) for (Database.Error error : resultErrors) errors.add(String.valueOf(error.getStatusCode()));
                    Id[] mergedIds = result.getMergedRecordIds();
                    details.add(new Map<String,Object>{'success'=>result.isSuccess(), 'masterMatches'=>result.getId()==master.Id, 'merged'=>mergedIds == null ? 0 : mergedIds.size(), 'errors'=>errors});
                }
                row.put('results',details);
                Set<Id> recordIds = new Set<Id>{master.Id,first.Id,second.Id};
                row.put('remaining',[SELECT COUNT() FROM Account WHERE Id IN :recordIds]);
            }
        } catch (Exception error) {
            row.put('exceptionType',error.getTypeName());
            row.put('exceptionMessage',error.getMessage());
        }
        return row;
    }
}
