@IsTest
public class Sp_l12_WhileUninitLocalEachIteration {
  static String outBuf = '';
  
  @IsTest static void t() {
    Integer n = 0;
    String seen = '';
    while (n < 2) { Integer abc; seen += String.valueOf(abc) + ','; abc = 5; n++; }
    outBuf += 'OUT:' + String.valueOf(seen) + ';';
    System.assert(false, outBuf);
  }
}
