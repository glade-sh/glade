@IsTest
public class Sp_d16_DottedBindUndeclared {
  static String outBuf = '';
  
  @IsTest static void t() {
    insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
    List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :acc.Name' + ' LIMIT 1');
    outBuf += 'OUT:' + String.valueOf(r.isEmpty() ? 'none' : r[0].Name) + ';';
    System.assert(false, outBuf);
  }
}
