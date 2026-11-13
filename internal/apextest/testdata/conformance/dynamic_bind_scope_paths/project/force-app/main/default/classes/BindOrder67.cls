public class BindOrder67 {
 public String field = 'before';
 public static String check() {
  insert new Account(Name='before'); insert new Account(Name='after');
  String value = 'before'; String query = 'SELECT Id FROM Account WHERE Name = :value'; String countQuery = 'SELECT COUNT() FROM Account WHERE Name = :value';
  value = 'after';
  Integer a = Database.query(query).size(); Integer b = Database.countQuery(countQuery); Integer c = Database.getQueryLocator(query).iterator().hasNext() ? 1 : 0;
  value = 'missing'; Integer d = Database.query(query).size(); Integer e = Database.countQuery(countQuery); Integer f = Database.getQueryLocator(query).iterator().hasNext() ? 1 : 0;
  BindOrder67 h = new BindOrder67(); Integer g = h.read(); h.field = 'missing'; Integer i = h.read();
  return a+','+b+','+c+','+d+','+e+','+f+','+g+','+i;
 }
 public Integer read() { return Database.countQuery('SELECT COUNT() FROM Account WHERE Name = :field'); }
}