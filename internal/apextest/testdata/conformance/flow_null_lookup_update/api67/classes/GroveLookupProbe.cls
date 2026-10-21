public class GroveLookupProbe {
 public static String observe() {
  Contact target = new Contact(LastName='Grove found'); insert target;
  Map<String,Object> result = new Map<String,Object>();
  for (String row : new List<String>{'N01','N02'}) {
   Map<String,Object> cell = new Map<String,Object>{'exception'=>null,'description'=>null};
   try {
    insert new Account(Name=row=='N01'?'Grove missing':'Grove found');
    Contact stored=[SELECT Description FROM Contact WHERE Id=:target.Id];
    cell.put('description',stored.Description);
   } catch (Exception error) { cell.put('exception',error.getTypeName()); }
   result.put(row,cell);
  }
  return JSON.serialize(result);
 }
}
