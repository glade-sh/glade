public class NetworkLiteralObservation {
 public static String observe() {
  Map<String,String> rows = new Map<String,String>();
  try { Network missing = [SELECT Id FROM Network WHERE Name = 'Missing Synthetic Portal']; rows.put('missingLiteral', 'NO_EXCEPTION'); }
  catch (Exception e) { rows.put('missingLiteral', e.getTypeName() + '|' + e.getMessage()); }
  try { List<Id> ids = new List<Id>{UserInfo.getUserId()}; Network missing = [SELECT Id FROM Network WHERE Id = :ids[0]]; rows.put('missingIndexed', 'NO_EXCEPTION'); }
  catch (Exception e) { rows.put('missingIndexed', e.getTypeName() + '|' + e.getMessage()); }
  try { String name = [SELECT Name FROM Network WHERE Name = 'Missing Synthetic Portal'].Name; rows.put('missingField', 'NO_EXCEPTION'); }
  catch (Exception e) { rows.put('missingField', e.getTypeName() + '|' + e.getMessage()); }
  rows.put('safeMissing', String.valueOf([SELECT Name FROM Network WHERE Name = 'Missing Synthetic Portal']?.Name));
  rows.put('limitZero', String.valueOf([SELECT Id FROM Network LIMIT 0].size()));
  List<Network> records = [SELECT Id, Name FROM Network WHERE Name = 'Synthetic Portal'];
  rows.put('literalCount', String.valueOf(records.size()));
  if (!records.isEmpty()) { Network record = [SELECT Id, Name FROM Network WHERE Name = 'Synthetic Portal']; rows.put('literalName', record.Name); }
  return JSON.serialize(rows);
 }
}
