@IsTest
public class Sp_n06_CalleeDoesNotSeeCallerLocal {
  static String outBuf = '';
  static String callee() { try { List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1'); return r.isEmpty() ? 'none' : r[0].Name; } catch (Exception e) { return e.getTypeName() + ': ' + e.getMessage(); } }
  @IsTest static void t() {
    insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
    String n = 'bound';
    outBuf += 'OUT:' + String.valueOf(callee()) + ';';
    System.assert(false, outBuf);
  }
}
