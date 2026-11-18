@IsTest
public class Sp_l15_IfLocalThenRedeclare {
  static String outBuf = '';
  
  @IsTest static void t() {
    if (true) { Integer abc = 1; }
    Integer abc = 7;
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
