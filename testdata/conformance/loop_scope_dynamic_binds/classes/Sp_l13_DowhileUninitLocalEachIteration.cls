@IsTest
public class Sp_l13_DowhileUninitLocalEachIteration {
  static String outBuf = '';
  
  @IsTest static void t() {
    Integer n = 0;
    String seen = '';
    do { Integer abc; seen += String.valueOf(abc) + ','; abc = 5; n++; } while (n < 2);
    outBuf += 'OUT:' + String.valueOf(seen) + ';';
    System.assert(false, outBuf);
  }
}
