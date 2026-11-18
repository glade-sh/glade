@IsTest
public class Sp_f03_FieldAfterFor {
  static String outBuf = '';
  static Integer abc = 100;
  @IsTest static void t() {
    for (Integer i = 0; i < 1; i++) { Integer abc = 1; }
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
