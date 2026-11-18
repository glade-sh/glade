@IsTest
public class Sp_f01_FieldAfterWhile {
  static String outBuf = '';
  static Integer abc = 100;
  @IsTest static void t() {
    Boolean c = true;
    while (c) { Integer abc = 1; c = false; }
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
