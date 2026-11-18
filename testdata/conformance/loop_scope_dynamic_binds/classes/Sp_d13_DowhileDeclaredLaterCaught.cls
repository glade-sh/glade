@IsTest
public class Sp_d13_DowhileDeclaredLaterCaught {
  static String outBuf = '';
  
  @IsTest static void t() {
    insert new List<Account>{new Account(Name='bound'), new Account(Name='fld'), new Account(Name='loc')};
    Integer k = 0;
    do {
     try { List<Account> r = Database.query('SELECT Name FROM Account WHERE Name = :n' + ' LIMIT 1'); outBuf += 'OUT:' + String.valueOf(r.isEmpty() ? 'none' : r[0].Name) + ';'; } catch (Exception e) { outBuf += 'OUT:' + String.valueOf(e.getTypeName() + ': ' + e.getMessage()) + ';'; }
     String n = 'bound';
     k++;
    } while (k < 3);
    System.assert(false, outBuf);
  }
}
