public class MapleLookupIdProbe {
 public static String observe() {
  String bad='not-a-reference-value';
  Map<String,String> rows=new Map<String,String>();
  try { Contact row = new Contact(AccountId=bad); rows.put('constructorLookup','accepted|'+String.valueOf(row.get('AccountId'))); }
  catch(Exception e) { rows.put('constructorLookup',e.getTypeName()+'|'+e.getMessage()); }
  try { Contact row = new Contact(); row.AccountId=bad; rows.put('assignedLookup','accepted|'+String.valueOf(row.get('AccountId'))); }
  catch(Exception e) { rows.put('assignedLookup',e.getTypeName()+'|'+e.getMessage()); }
  try { SObject row = new Contact(); row.put('AccountId',bad); rows.put('dynamicLookup','accepted|'+String.valueOf(row.get('AccountId'))); }
  catch(Exception e) { rows.put('dynamicLookup',e.getTypeName()+'|'+e.getMessage()); }
  try { SObject row = new Contact(); row.put(Contact.AccountId,bad); rows.put('tokenLookup','accepted|'+String.valueOf(row.get('AccountId'))); }
  catch(Exception e) { rows.put('tokenLookup',e.getTypeName()+'|'+e.getMessage()); }
  try { Contact row = new Contact(Id=bad); rows.put('constructorId','accepted|'+String.valueOf(row.get('Id'))); }
  catch(Exception e) { rows.put('constructorId',e.getTypeName()+'|'+e.getMessage()); }
  try { Contact row = new Contact(); row.Id=bad; rows.put('assignedId','accepted|'+String.valueOf(row.get('Id'))); }
  catch(Exception e) { rows.put('assignedId',e.getTypeName()+'|'+e.getMessage()); }
  try { SObject row = new Contact(); row.put('Id',bad); rows.put('dynamicId','accepted|'+String.valueOf(row.get('Id'))); }
  catch(Exception e) { rows.put('dynamicId',e.getTypeName()+'|'+e.getMessage()); }
  try { Contact row = new Contact(AccountId=null); rows.put('nullLookup','accepted|'+String.valueOf(row.get('AccountId'))); }
  catch(Exception e) { rows.put('nullLookup',e.getTypeName()+'|'+e.getMessage()); }
  try { Contact row = new Contact(AccountId=''); rows.put('emptyLookup','accepted|'+String.valueOf(row.get('AccountId'))); }
  catch(Exception e) { rows.put('emptyLookup',e.getTypeName()+'|'+e.getMessage()); }
  try { Contact row = new Contact(AccountId='001000000000001'); rows.put('opaqueLookup','accepted|'+String.valueOf(row.get('AccountId'))); }
  catch(Exception e) { rows.put('opaqueLookup',e.getTypeName()+'|'+e.getMessage()); }
  try { Contact row = new Contact(AccountId=UserInfo.getUserId()); rows.put('wrongTargetLookup','accepted|'+String.valueOf(row.get('AccountId')==UserInfo.getUserId())); }
  catch(Exception e) { rows.put('wrongTargetLookup',e.getTypeName()+'|'+e.getMessage()); }
  return JSON.serialize(rows);
 }
}
