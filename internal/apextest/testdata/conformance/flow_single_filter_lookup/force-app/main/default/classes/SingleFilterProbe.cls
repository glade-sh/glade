public class SingleFilterProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> r=new Map<String,Object>();
  Account a=new Account(Name='river'); insert a;
  r.put('empty',[SELECT Description FROM Account WHERE Id=:a.Id].Description);
  insert new Account(Name='needle');
  Account b=new Account(Name='lake'); insert b;
  r.put('found',[SELECT Description FROM Account WHERE Id=:b.Id].Description);
  a.Name='mountain'; update a;
  r.put('updated',[SELECT Description FROM Account WHERE Id=:a.Id].Description);
  return r;
 }
}
