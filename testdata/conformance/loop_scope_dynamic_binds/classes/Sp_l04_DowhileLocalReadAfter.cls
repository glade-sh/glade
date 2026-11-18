@IsTest
public class Sp_l04_DowhileLocalReadAfter {
  static String outBuf = '';
  
  @IsTest static void t() {
    Boolean c = true;
    do { Integer abc = 1; c = false; } while (c);
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
