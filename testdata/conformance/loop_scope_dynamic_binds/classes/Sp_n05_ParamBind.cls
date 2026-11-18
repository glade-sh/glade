@IsTest
public class Sp_n05_ParamBind {
  static String outBuf = '';
  static String run(String n) { List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1'); return r.isEmpty() ? 'none' : r[0].Name; }
  @IsTest static void t() {
    insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
    outBuf += 'OUT:' + String.valueOf(run('bound')) + ';';
    System.assert(false, outBuf);
  }
}
