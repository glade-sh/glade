public class StandardNoRowsProbe {
 public static String sample() {
  Map<String,Object> out=new Map<String,Object>();
  Case c=new Case(Subject='Synthetic present',Status='New',Origin='Phone'); insert c;
  out.put('existingCase',[SELECT COUNT() FROM Case WHERE Id=:c.Id]);
  out.put('missingCases',[SELECT COUNT() FROM Case WHERE Subject='Synthetic absent']);
  try { Case absent=[SELECT Id FROM Case WHERE Subject='Synthetic absent' LIMIT 1]; out.put('caseSingleton','found'); }
  catch(QueryException e) { out.put('caseSingleton',e.getTypeName()); }
  out.put('existingPermission',[SELECT COUNT() FROM CustomPermission WHERE DeveloperName='Synthetic_Present']);
  out.put('missingPermission',[SELECT COUNT() FROM CustomPermission WHERE DeveloperName='Synthetic_Absent']);
  try { CustomPermission p=[SELECT Id FROM CustomPermission WHERE DeveloperName='Synthetic_Absent']; out.put('permissionSingleton','found'); }
  catch(QueryException e) { out.put('permissionSingleton',e.getTypeName()); }
  return JSON.serialize(out);
 }
}
