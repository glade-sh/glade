public class BoundaryMagnitudeObservation { public static List<Object> observe() {
Savepoint point=Database.setSavepoint();
try {
List<Object> rows=new List<Object>();
for(Decimal value:new List<Decimal>{99.1,99.49,99.5,-99.1,-99.49,-99.5}) {
 Account record=new Account(Name='Boundary control',MagnitudeSmall__c=value);
 Database.SaveResult result=Database.insert(record,false,AccessLevel.SYSTEM_MODE);
 Map<String,Object> row=new Map<String,Object>{'input'=>value,'success'=>result.isSuccess()};
 if(result.isSuccess()) {Id recordId=result.getId();row.put('stored',[SELECT MagnitudeSmall__c FROM Account WHERE Id=:recordId WITH SYSTEM_MODE].MagnitudeSmall__c);}
 else {row.put('code',String.valueOf(result.getErrors()[0].getStatusCode()));row.put('fields',result.getErrors()[0].getFields());row.put('message',result.getErrors()[0].getMessage());}
 rows.add(row);
}
return rows;

} finally {Database.rollback(point);}
} }
