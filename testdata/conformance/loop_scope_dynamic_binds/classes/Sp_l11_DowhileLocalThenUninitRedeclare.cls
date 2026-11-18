@IsTest
public class Sp_l11_DowhileLocalThenUninitRedeclare {
  static String outBuf = '';
  
  @IsTest static void t() {
    Boolean c = true;
    do { Integer abc = 1; c = false; } while (c);
    Integer abc;
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
