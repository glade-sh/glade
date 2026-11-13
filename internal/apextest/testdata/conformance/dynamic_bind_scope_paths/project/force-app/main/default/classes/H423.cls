public class H423 {
  public String f = 'x';
  public static String s = 'x';
  public static List<Account> run() { return Database.query('SELECT Id FROM Account WHERE Name = :callerLocal'); }
  public static List<Account> withParam(String p) { return Database.query('SELECT Id FROM Account WHERE Name = :p'); }
  public List<Account> byField() { return Database.query('SELECT Id FROM Account WHERE Name = :f'); }
  public static List<Account> byStatic() { return Database.query('SELECT Id FROM Account WHERE Name = :s'); }
  public List<Account> viaThis() { return Database.query('SELECT Id FROM Account WHERE Name = :this.f'); }
}
