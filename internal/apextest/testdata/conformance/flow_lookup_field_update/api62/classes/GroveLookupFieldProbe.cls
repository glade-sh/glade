public class GroveLookupFieldProbe {
 public static String observe() {
  Account target=new Account(Name='Grove lookup hit');insert target;
  Map<String,Object> result=new Map<String,Object>();
  for (String key:new List<String>{'L01','L02','L03','L04'}) {
   Contact child=new Contact(LastName=key,AccountId=target.Id,FirstName=(key=='L02'||key=='L04')?'Grove lookup hit':'Grove lookup miss');
   Map<String,Object> cell=new Map<String,Object>{'isNull'=>false,'matchesTarget'=>false,'description'=>null,'exception'=>null};
   try { insert child; Contact stored=[SELECT AccountId,Description FROM Contact WHERE Id=:child.Id];
    cell.put('isNull',stored.AccountId==null);cell.put('matchesTarget',stored.AccountId==target.Id);cell.put('description',stored.Description);
   } catch (Exception error) {cell.put('exception',error.getTypeName());}
   result.put(key,cell);
  }
  return JSON.serialize(result);
 }
}
