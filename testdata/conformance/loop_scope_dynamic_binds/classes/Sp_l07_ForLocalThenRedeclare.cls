@IsTest
public class Sp_l07_ForLocalThenRedeclare {
  static String outBuf = '';
  
  @IsTest static void t() {
    for (Integer i = 0; i < 1; i++) { Integer abc = 1; }
    Integer abc = 7;
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
