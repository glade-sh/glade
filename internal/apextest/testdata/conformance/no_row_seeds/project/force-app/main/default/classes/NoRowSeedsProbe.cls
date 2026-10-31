public class NoRowSeedsProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> out = new Map<String,Object>();
  out.put('roleDefault', [SELECT count() FROM UserRole WHERE Name = 'Default']);
  out.put('roleMissing', [SELECT count() FROM UserRole WHERE Name = 'Glade Nonexistent Role 92711']);
  try { UserRole r = [SELECT Id FROM UserRole WHERE Name = 'Glade Nonexistent Role 92711' LIMIT 1]; out.put('missingException','none'); }
  catch (QueryException e) { out.put('missingException', e.getMessage()); }
  out.put('standardPricebook', [SELECT count() FROM Pricebook2 WHERE IsStandard = true]);
  out.put('defaultBusinessHours', [SELECT count() FROM BusinessHours WHERE IsDefault = true]);
  return out;
 }
}
