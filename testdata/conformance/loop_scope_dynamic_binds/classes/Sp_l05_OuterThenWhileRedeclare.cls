@IsTest
public class Sp_l05_OuterThenWhileRedeclare {
  static String outBuf = '';
  
  @IsTest static void t() {
    Integer abc = 7;
    Boolean c = true;
    while (c) { Integer abc = 1; c = false; }
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
