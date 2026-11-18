@IsTest
public class Sp_f02_FieldAfterDowhile {
  static String outBuf = '';
  static Integer abc = 100;
  @IsTest static void t() {
    Boolean c = true;
    do { Integer abc = 1; c = false; } while (c);
    outBuf += 'OUT:' + String.valueOf(abc) + ';';
    System.assert(false, outBuf);
  }
}
