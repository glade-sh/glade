public class Review349Probe {
    public static List<Object> run() {
        Savepoint sp = Database.setSavepoint();
        List<Object> rows = new List<Object>();
        for (List<String> pair : new List<List<String>>{new List<String>{'1.25','1.250'}, new List<String>{'-2.50','-2.500'}, new List<String>{'1.25','1.251'}}) {
            Database.delete([SELECT Id FROM ReviewNumber__c], AccessLevel.SYSTEM_MODE);
            ReviewNumber__c initial = new ReviewNumber__c(Name='Initial', ReviewKey__c=Decimal.valueOf(pair[0])); Database.insert(initial, AccessLevel.SYSTEM_MODE);
            ReviewNumber__c candidate = new ReviewNumber__c(Name='Candidate', ReviewKey__c=Decimal.valueOf(pair[1]));
            Database.UpsertResult outcome = Database.upsert(candidate, ReviewNumber__c.Fields.ReviewKey__c, false, AccessLevel.SYSTEM_MODE);
            rows.add(new Map<String,Object>{'case'=>'decimal','left'=>pair[0],'right'=>pair[1],'success'=>outcome.isSuccess(),'created'=>outcome.isCreated(),'same'=>initial.Id == candidate.Id});
        }
        for (RecordType rt : [SELECT Id, DeveloperName, IsPersonType FROM RecordType WHERE SobjectType='Account' AND DeveloperName IN ('Person_Commerce','Individual_Business') ORDER BY DeveloperName]) {
            Review349Trace.beforeKinds.clear();
            Account candidate = new Account(Name='Business case', RecordTypeId=rt.Id, PersonHomePhone='4155550123');
            Database.SaveResult result = Database.insert(candidate, false, AccessLevel.SYSTEM_MODE);
            rows.add(new Map<String,Object>{'case'=>'recordtype','name'=>rt.DeveloperName,'personType'=>rt.IsPersonType,'success'=>result.isSuccess(),'status'=>result.isSuccess()?'SUCCESS':String.valueOf(result.getErrors()[0].getStatusCode()),'beforeKinds'=>Review349Trace.beforeKinds.clone()});
            Review349Trace.beforeKinds.clear();
            Account blank = new Account(Name='Business boundary', RecordTypeId=rt.Id, PersonMobilePhone='   ');
            Database.SaveResult accepted = Database.insert(blank, false, AccessLevel.SYSTEM_MODE);
            rows.add(new Map<String,Object>{'case'=>'recordtype_blank','name'=>rt.DeveloperName,'personType'=>rt.IsPersonType,'success'=>accepted.isSuccess(),'beforeKinds'=>Review349Trace.beforeKinds.clone()});
        }
        Database.rollback(sp);
        return rows;
    }
}
