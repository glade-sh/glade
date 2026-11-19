@IsTest private class Ld11NoGrantTest {
 @IsTest static void observe() {
  Map<String,Object> o=new Map<String,Object>();
  try { o.put('countQuery',[SELECT count() FROM Ungranted__c]); } catch(Exception e) { o.put('countQuery',LdProbe.error(e)); }
  try { List<Ungranted__c> rows=[SELECT Value__c FROM Ungranted__c LIMIT 1]; o.put('fieldQuery',rows.size()); }
  catch(Exception e) { o.put('fieldQuery',LdProbe.error(e)); }
  try { insert new Ungranted__c(Name='u1',Value__c='v1'); o.put('insert','ok'); }
  catch(Exception e) { o.put('insert',LdProbe.error(e)); }
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
