@IsTest
public class Sp_n02_FieldBindAfterWhileLocal {
  static String outBuf = '';
  static String n = 'fld';
  @IsTest static void t() {
    insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
    Boolean c = true;
    while (c) { String n = 'loc'; c = false; }
    List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1');
    outBuf += 'OUT:' + String.valueOf(r.isEmpty() ? 'none' : r[0].Name) + ';';
    System.assert(false, outBuf);
  }
}
