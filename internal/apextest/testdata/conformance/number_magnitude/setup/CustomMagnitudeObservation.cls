public class CustomMagnitudeObservation {
    private static Map<String,Object> save(String field, Decimal value) {
        Account record = new Account(Name='Magnitude observation');
        record.put(field,value);
        Database.SaveResult result = Database.insert(record,false,AccessLevel.SYSTEM_MODE);
        Map<String,Object> row = new Map<String,Object>{'input'=>value,'success'=>result.isSuccess()};
        if (result.isSuccess()) {
            Id recordId=result.getId();
            List<SObject> stored=Database.query('SELECT '+field+' FROM Account WHERE Id=:recordId',AccessLevel.SYSTEM_MODE);
            row.put('stored',stored[0].get(field));
        } else {
            row.put('code',String.valueOf(result.getErrors()[0].getStatusCode()));
            row.put('fields',result.getErrors()[0].getFields());
            row.put('message',result.getErrors()[0].getMessage());
        }
        return row;
    }
    public static Map<String,Object> observe() {
        Savepoint point=Database.setSavepoint();
        try {
            Map<String,Object> rows=new Map<String,Object>();
            rows.put('smallBoundary',save('MagnitudeSmall__c',99));
            rows.put('smallBoundaryOverflow',save('MagnitudeSmall__c',100));
            rows.put('smallOverflow',save('MagnitudeSmall__c',1000));
            rows.put('smallNegativeOverflow',save('MagnitudeSmall__c',-1000));
            rows.put('mediumBoundary',save('MagnitudeMedium__c',999999));
            rows.put('mediumOverflow',save('MagnitudeMedium__c',1000000));
            rows.put('mediumNegativeOverflow',save('MagnitudeMedium__c',-1000000));
            rows.put('extraFraction',save('MagnitudeSmall__c',12.3456789));
            rows.put('boundaryFraction',save('MagnitudeSmall__c',99.9999999));
            rows.put('nullControl',save('MagnitudeSmall__c',null));
            rows.put('zeroControl',save('MagnitudeSmall__c',0));
            List<Account> mixed=new List<Account>{new Account(Name='Valid',MagnitudeSmall__c=12.3456789),new Account(Name='Invalid',MagnitudeSmall__c=1000)};
            List<Database.SaveResult> partial=Database.insert(mixed,false,AccessLevel.SYSTEM_MODE);
            rows.put('partial',new Map<String,Object>{'validSuccess'=>partial[0].isSuccess(),'invalidSuccess'=>partial[1].isSuccess(),'invalidCode'=>partial[1].isSuccess()?null:String.valueOf(partial[1].getErrors()[0].getStatusCode())});
            Account seed=new Account(Name='Update control',MagnitudeMedium__c=12.3456789);
            insert as system seed;
            seed.MagnitudeMedium__c=1000000;
            Database.SaveResult updateResult=Database.update(seed,false,AccessLevel.SYSTEM_MODE);
            rows.put('update',new Map<String,Object>{'success'=>updateResult.isSuccess(),'stored'=>[SELECT MagnitudeMedium__c FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE].MagnitudeMedium__c,'code'=>updateResult.isSuccess()?null:String.valueOf(updateResult.getErrors()[0].getStatusCode())});
            try {
                insert as system new Account(Name='Statement control',MagnitudeSmall__c=1000);
                rows.put('statement',new Map<String,Object>{'caught'=>false});
            } catch (DmlException failure) {
                rows.put('statement',new Map<String,Object>{'caught'=>true,'type'=>failure.getTypeName(),'code'=>String.valueOf(failure.getDmlType(0)),'fields'=>failure.getDmlFieldNames(0),'message'=>failure.getDmlMessage(0)});
            }
            return rows;
        } finally { Database.rollback(point); }
    }
}
