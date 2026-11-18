@IsTest
public class Sp_d12_WhileDeclaredLaterCaught {
  static String outBuf = '';
  
  @IsTest static void t() {
    insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
    Integer k = 0;
    while (k < 3) {
     try { List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1'); outBuf += 'OUT:' + String.valueOf(r.isEmpty() ? 'none' : r[0].Name) + ';'; } catch (Exception e) { outBuf += 'OUT:' + String.valueOf(e.getTypeName() + ': ' + e.getMessage()) + ';'; }
     String n = 'bound';
     k++;
    }
    System.assert(false, outBuf);
  }
}
