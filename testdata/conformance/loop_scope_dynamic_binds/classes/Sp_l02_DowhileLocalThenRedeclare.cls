@IsTest
public class Sp_l02_DowhileLocalThenRedeclare {
  static String outBuf = '';
  
  @IsTest static void t() {
    Boolean c = true;
    do { Integer abc = 1; c = false; } while (c);
    Integer abc = 7;
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
