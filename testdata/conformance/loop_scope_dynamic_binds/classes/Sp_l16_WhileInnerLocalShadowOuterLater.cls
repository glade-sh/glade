@IsTest
public class Sp_l16_WhileInnerLocalShadowOuterLater {
  static String outBuf = '';
  
  @IsTest static void t() {
    Boolean c = true;
    while (c) { Integer abc = 1; c = false; }
    String abc;
    abc = 'x';
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
