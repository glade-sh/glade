public class NetworkObservation {
 public static String observe() {
  Map<String,String> rows = new Map<String,String>();
  rows.put('available', String.valueOf(Schema.getGlobalDescribe().containsKey('Network')));
  if (!Schema.getGlobalDescribe().containsKey('Network')) return JSON.serialize(rows);
  rows.put('count', String.valueOf(Database.countQuery('SELECT COUNT() FROM Network')));
  List<SObject> records = Database.query('SELECT Id, Name, Status, UrlPathPrefix, OptionsGuestChatterEnabled, SelfRegProfileId FROM Network ORDER BY Name');
  if (!records.isEmpty()) {
   SObject record = records[0];
   rows.put('name', String.valueOf(record.get('Name')));
   rows.put('status', String.valueOf(record.get('Status')));
   rows.put('prefix', String.valueOf(record.get('UrlPathPrefix')));
   rows.put('idPrefix', String.valueOf(record.Id).left(3));
   rows.put('guestChatter', String.valueOf(record.get('OptionsGuestChatterEnabled')));
   rows.put('selfRegNull', String.valueOf(record.get('SelfRegProfileId') == null));
  }
  rows.put('missingList', String.valueOf(Database.query('SELECT Id FROM Network WHERE Name = \'Missing Synthetic Portal\'').size()));
  rows.put('missingCount', String.valueOf(Database.countQuery('SELECT COUNT() FROM Network WHERE Name = \'Missing Synthetic Portal\'')));
  try { SObject missing = Database.query('SELECT Id FROM Network WHERE Name = \'Missing Synthetic Portal\''); rows.put('missingDynamic', 'NO_EXCEPTION'); }
  catch (Exception e) { rows.put('missingDynamic', e.getTypeName() + '|' + e.getMessage()); }
  return JSON.serialize(rows);
 }
}
