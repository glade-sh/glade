@IsTest
public class Sp_f04_FieldAfterIf {
  static String outBuf = '';
  static Integer abc = 100;
  @IsTest static void t() {
    if (true) { Integer abc = 1; }
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
