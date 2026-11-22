@IsTest private class MatcherReplaceTest {
 @IsTest static void observe() {
String a = Pattern.compile('').matcher('a\uD83D\uDE00b').replaceAll('-');
String b = Pattern.compile('x*').matcher('\uD83D\uDE00').replaceAll('-');
String observation = String.valueOf(a.length()) + '|' + a.replace('\uD83D','H').replace('\uDE00','L') + '|' + String.valueOf(b.length());
System.assertEquals('ROW', observation);
 }
}
