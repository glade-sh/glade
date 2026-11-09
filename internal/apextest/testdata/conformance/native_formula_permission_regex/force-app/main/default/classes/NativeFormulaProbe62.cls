public class NativeFormulaProbe62 {
 public static List<String> current() {
  Map<String,Object> row = new Map<String,Object>();
  List<String> order = new List<String>();
  put(order, row, 'hasBypass', FeatureManagement.checkPermission('Probe_Bypass'));
  for (String phone : new List<String>{'5551234567','555123456','55512345678','555-123-4567','55512345ab'}) {
   put(order, row, 'phone ' + phone, save(new ProbeRecord__c(Name='phone', Phone__c=phone)));
   put(order, row, 'contact ' + phone, save(new Contact(LastName='Probe', Phone=phone)));
  }
  put(order, row, 'phone blank', save(new ProbeRecord__c(Name='phone')));
  put(order, row, 'guarded', save(new ProbeRecord__c(Name='guarded')));
  put(order, row, 'bypassOnly', save(new ProbeRecord__c(Name='bypass-only')));
  put(order, row, 'ordinary', save(new ProbeRecord__c(Name='ordinary')));
  for (String code : new List<String>{'AB123','AB12345','ab123','xAB123','AB123x','AB12',' AB123'}) {
   put(order, row, 'shape ' + code, save(new ProbeRecord__c(Name='shape', Code__c=code)));
  }
  for (String code : new List<String>{'123','12345','ab123','12'}) {
   put(order, row, 'digits ' + code, save(new ProbeRecord__c(Name='digits', Code__c=code)));
  }
  for (String code : new List<String>{'abc-12','abc-1','abc-123','ABC-12','abc-12x','x-12'}) {
   put(order, row, 'anchored ' + code, save(new ProbeRecord__c(Name='anchored', Code__c=code)));
  }
  List<String> lines = new List<String>();
  for (String key : order) { lines.add(key + '=' + row.get(key)); }
  return lines;
 }
 static void put(List<String> order, Map<String,Object> row, String key, Object value) { order.add(key); row.put(key, value); }
 public static String save(SObject record) {
  Database.SaveResult result = Database.insert(record, false);
  if (result.isSuccess()) { return 'ok'; }
  String code = String.valueOf(result.getErrors()[0].getStatusCode());
  return code == 'FIELD_CUSTOM_VALIDATION_EXCEPTION' ? result.getErrors()[0].getMessage() : code + ':' + result.getErrors()[0].getMessage();
 }
}
