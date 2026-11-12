@IsTest
public class Sp_n01_FieldBindLocalLater {
  static String outBuf = '';
  static String n = 'fld';
  @IsTest static void t() {
    insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
    List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1');
    outBuf += 'OUT:' + String.valueOf(r.isEmpty() ? 'none' : r[0].Name) + ';';
    String n = 'loc';
    System.assert(false, outBuf);
  }
}
