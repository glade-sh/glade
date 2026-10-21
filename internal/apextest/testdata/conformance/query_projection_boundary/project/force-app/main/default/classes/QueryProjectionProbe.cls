public class QueryProjectionProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> out=new Map<String,Object>();
  out.put('userInfoName',UserInfo.class.getName());
  Object numbers=Type.forName('Map<String,Double>').newInstance();
  out.put('doubleMapIsInteger',numbers instanceof Map<String,Integer>);
  out.put('doubleMapIsDouble',numbers instanceof Map<String,Double>);
  Object emptyText=new Map<String,String>();
  out.put('emptyTextIsRecordMap',emptyText instanceof Map<Integer,Account>);
  Object populated=new Map<String,Object>{'key'=>'value'};
  out.put('populatedIsTextMap',populated instanceof Map<String,String>);
  out.put('populatedIsIntegerKeyMap',populated instanceof Map<Integer,String>);
  out.put('populatedIsRecordMap',populated instanceof Map<String,Account>);
  out.put('populatedIsObjectMap',populated instanceof Map<String,Object>);
  Account parent=new Account(Name='Owned projection',Rating='Warm'); insert parent;
  insert new Contact(LastName='Owned child',AccountId=parent.Id);
  Id parentId=parent.Id;
  try {
   List<SObject> groups=Database.query('SELECT Rating ratingKey, COUNT(Id) tally FROM Account WHERE Id = :parentId GROUP BY Rating',AccessLevel.USER_MODE);
   out.put('aliasCount',groups.size()); out.put('aliasValue',groups[0].get('ratingKey')); out.put('aliasTally',groups[0].get('tally'));
  } catch(Exception e) { out.put('aliasError',e.getTypeName()+':'+e.getMessage()); }
  Account queried=[SELECT Id,Name,(SELECT Id FROM Contacts) FROM Account WHERE Id=:parent.Id];
  queried.Name='Owned revised';
  try {
   List<Database.UpsertResult> result=Database.upsert(new List<Account>{queried},AccessLevel.USER_MODE);
   out.put('childQueryUpsert',result[0].isSuccess()); out.put('updatedName',[SELECT Name FROM Account WHERE Id=:parent.Id].Name);
  } catch(Exception e) { out.put('childQueryError',e.getTypeName()+':'+e.getMessage()); }
  return out;
 }
}
