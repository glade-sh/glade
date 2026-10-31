public class RouteProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> r = new Map<String,Object>();
  Account t = new Account(Name='RouteLoopTrigger'); insert t;
  r.put('empty', [SELECT Description FROM Account WHERE Id=:t.Id].Description);
  Account p = new Account(Name='RouteParent'); insert p;
  Account s = new Account(Name='RouteLoopSeed', ParentId=p.Id, Phone='yes'); insert s;
  t = new Account(Name='RouteLoopTrigger'); insert t;
  r.put('parent', [SELECT Description FROM Account WHERE Id=:t.Id].Description);
  s.ParentId=null; update s;
  t = new Account(Name='RouteLoopTrigger'); insert t;
  r.put('noParent', [SELECT Description FROM Account WHERE Id=:t.Id].Description);
  s.Phone=null; update s;
  t = new Account(Name='RouteLoopTrigger'); insert t;
  r.put('noMatch', [SELECT Description FROM Account WHERE Id=:t.Id].Description);
  return r;
 }
}
