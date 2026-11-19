@IsTest
public class Sp_l14_WhileLocalTypeChangeAfter {
  static String outBuf = '';
  
  @IsTest static void t() {
    Boolean c = true;
    while (c) { Integer abc = 1; c = false; }
    String abc = 'str';
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
