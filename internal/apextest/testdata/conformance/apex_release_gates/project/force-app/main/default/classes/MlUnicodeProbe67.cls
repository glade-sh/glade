public class MlUnicodeProbe67 {
  public static String run(){
    String s='''
 a \u0041 b
''';
    return 'MU1 ['+s.replace('\n','~')+']';
  }
}
