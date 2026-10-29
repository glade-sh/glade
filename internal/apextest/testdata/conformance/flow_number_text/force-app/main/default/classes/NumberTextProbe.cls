public class NumberTextProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> r=new Map<String,Object>();
  List<Product2> rows=new List<Product2>{new Product2(Name='River',Cycle__c=2031,Tone__c='Amber'),new Product2(Name='Lake',Cycle__c=0,Tone__c='Blue'),new Product2(Name='Hill')};
  insert rows;
  for(Product2 p:[SELECT Name,Description,ProductCode FROM Product2 WHERE Id IN :rows]) {r.put(p.Name,p.Description);r.put(p.Name+'Tone',p.ProductCode);}
  rows[0].Cycle__c=2042; rows[0].Tone__c=null; update rows[0];
  Product2 changed=[SELECT Description,ProductCode FROM Product2 WHERE Id=:rows[0].Id]; r.put('changed',changed.Description); r.put('changedTone',changed.ProductCode);
  return r;
 }
}
