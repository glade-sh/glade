public class AccountActiveProbe {
 public static String observe() {
  Map<String,Object> result=new Map<String,Object>();
  Map<String,Schema.SObjectField> fields=Schema.SObjectType.Account.fields.getMap();
  Map<String,Object> describe=new Map<String,Object>{'available'=>fields.containsKey('IsActive')};
  if (fields.containsKey('IsActive')) {
   Schema.DescribeFieldResult d=fields.get('IsActive').getDescribe();
   describe.put('createable',d.isCreateable()); describe.put('updateable',d.isUpdateable());
   describe.put('type',String.valueOf(d.getType())); describe.put('default',d.getDefaultValue());
   describe.put('nillable',d.isNillable());
  }
  describe.put('deletedWriteable',fields.get('IsDeleted').getDescribe().isUpdateable());
  result.put('D01',describe);
  Map<String,Object> direct=new Map<String,Object>{'exception'=>null};
  try {
   Account fresh=new Account(Name='Arbor direct default'); insert fresh;
   direct.put('default',readActive(fresh.Id));
   Account explicit=new Account(Name='Arbor direct explicit'); explicit.put('IsActive',true); insert explicit;
   direct.put('insert',readActive(explicit.Id));
   explicit.put('IsActive',false); update explicit; direct.put('update',readActive(explicit.Id));
  } catch(Exception e) { direct.put('exception',e.getTypeName()); }
  result.put('W01',direct);
  Map<String,Object> before=new Map<String,Object>{'exception'=>null};
  try {
   Account row=new Account(Name='Arbor before enable'); insert row; before.put('insert',readActive(row.Id));
   row.Name='Arbor before disable'; update row; before.put('update',readActive(row.Id));
  } catch(Exception e) { before.put('exception',e.getTypeName()); }
  result.put('B01',before);
  Map<String,Object> related=new Map<String,Object>{'exception'=>null};
  try {
   Account row=new Account(Name='Arbor related'); insert row;
   Contact child=new Contact(LastName='Arbor related enable',AccountId=row.Id); insert child;
   related.put('insert',readActive(row.Id));
   child.LastName='Arbor related disable'; update child; related.put('update',readActive(row.Id));
  } catch(Exception e) { related.put('exception',e.getTypeName()); }
  result.put('R01',related);
  Map<String,Object> missing=new Map<String,Object>{'exception'=>null,'inserted'=>false};
  try { Contact child=new Contact(LastName='Arbor related enable'); insert child; missing.put('inserted',child.Id!=null); }
  catch(Exception e) { missing.put('exception',e.getTypeName()); }
  result.put('N01',missing);
  return JSON.serialize(result);
 }
 private static Object readActive(Id recordId) {
  return Database.query('SELECT IsActive FROM Account WHERE Id = :recordId')[0].get('IsActive');
 }
}
