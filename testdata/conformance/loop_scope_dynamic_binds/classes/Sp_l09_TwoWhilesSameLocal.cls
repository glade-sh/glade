@IsTest
public class Sp_l09_TwoWhilesSameLocal {
  static String outBuf = '';
  
  @IsTest static void t() {
    Boolean c = true;
    while (c) { Integer abc = 1; c = false; }
    c = true;
    while (c) { Integer abc = 2; c = false; }
    outBuf += 'OUT:' + String.valueOf('ok') + ';';
    System.assert(false, outBuf);
  }
}
