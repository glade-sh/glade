public class ExternalUserAccessProbe67 {
 public static String observe() {
  Map<String, String> rows = new Map<String, String>();
  try { String result = Network.createExternalUserAsync(new User(), new Contact(), new Account()); rows.put('empty', result == null ? 'NULL' : 'VALUE|' + result); } catch (Exception e) { rows.put('empty', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { String result = Network.createExternalUserAsync(new User(Username='visitor@example.invalid', Email='visitor@example.invalid', LastName='Visitor', Alias='visit'), new Contact(LastName='Visitor'), new Account(Name='Visitor account')); rows.put('populated', result == null ? 'NULL' : 'VALUE|' + result); } catch (Exception e) { rows.put('populated', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { String result = Network.createExternalUserAsync(null, null, null); rows.put('null', result == null ? 'NULL' : 'VALUE|' + result); } catch (Exception e) { rows.put('null', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { String result = System.Network.createExternalUserAsync(new User(), new Contact(), new Account()); rows.put('qualified_empty', result == null ? 'NULL' : 'VALUE|' + result); } catch (Exception e) { rows.put('qualified_empty', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  return JSON.serialize(rows);
 }
}
