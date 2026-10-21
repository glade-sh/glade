public class GroveCreatorProbe {
 public static String observe() {
  String expected=[SELECT Email FROM User WHERE Id=:UserInfo.getUserId()].Email;
  Map<String,Object> result=new Map<String,Object>();
  for (String key:new List<String>{'C01','C02','C03'}) {
   Contact child=new Contact(LastName=key);
   Map<String,Object> cell=new Map<String,Object>{'matchesCreator'=>false,'exception'=>null};
   try { insert child; Contact stored=[SELECT Description FROM Contact WHERE Id=:child.Id];
    cell.put('matchesCreator',stored.Description==(key=='C02'?expected+expected:expected));
   } catch (Exception error) { cell.put('exception',error.getTypeName()); }
   result.put(key,cell);
  }
  return JSON.serialize(result);
 }
}
