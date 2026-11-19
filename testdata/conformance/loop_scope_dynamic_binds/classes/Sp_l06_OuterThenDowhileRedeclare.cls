@IsTest
public class Sp_l06_OuterThenDowhileRedeclare {
  static String outBuf = '';
  
  @IsTest static void t() {
    Integer abc = 7;
    Boolean c = true;
    do { Integer abc = 1; c = false; } while (c);
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
