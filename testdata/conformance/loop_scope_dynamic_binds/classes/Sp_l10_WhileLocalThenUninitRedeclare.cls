@IsTest
public class Sp_l10_WhileLocalThenUninitRedeclare {
  static String outBuf = '';
  
  @IsTest static void t() {
    Boolean c = true;
    while (c) { Integer abc = 1; c = false; }
    Integer abc;
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
