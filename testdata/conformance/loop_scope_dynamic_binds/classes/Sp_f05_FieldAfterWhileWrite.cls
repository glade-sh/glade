@IsTest
public class Sp_f05_FieldAfterWhileWrite {
  static String outBuf = '';
  static Integer abc = 100;
  @IsTest static void t() {
    Boolean c = true;
    while (c) { Integer abc = 1; c = false; }
    abc = 5;
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    outBuf += 'OUT:' + String.valueOf(Sp_f05_FieldAfterWhileWrite.abc) + ';';
    System.assert(false, outBuf);
  }
}
