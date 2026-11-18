@IsTest
public class Sp_l08_WhileLocalEachIteration {
  static String outBuf = '';
  
  @IsTest static void t() {
    Integer n = 0;
    while (n < 2) { Integer abc = n; n++; }
    outBuf += 'OUT:' + String.valueOf(n) + ';';
    System.assert(false, outBuf);
  }
}
