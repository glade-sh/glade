@IsTest private class CaptureGetterShapes67 {
 static Integer visits = 0;
 static Integer intValue { get { visits++; if (visits==1) intHelper(); return 7; } set; }
 static Integer intHelper() { return intValue; }
 static Boolean boolValue { get { visits++; if (visits==1) boolHelper(); return true; } set; }
 static Boolean boolHelper() { return boolValue; }
 static List<String> listValue { get { visits++; if (visits==1) listHelper(); return new List<String>{'x'}; } set; }
 static List<String> listHelper() { return listValue; }
 static Account objectValue { get { visits++; if (visits==1) objectHelper(); return new Account(Name='x'); } set; }
 static Account objectHelper() { return objectValue; }
 static String noSetter { get { visits++; if (visits==1) noSetterHelper(); return 'x'; } }
 static String noSetterHelper() { return noSetter; }
 static String direct { get { visits++; if (visits==1) { String resultValue=direct; } return 'x'; } set; }
 @IsTest static void integerHelper() { Integer resultValue=intValue; System.assertEquals(7,resultValue); System.assertEquals(2,visits); }
 @IsTest static void booleanHelper() { Boolean resultValue=boolValue; System.assertEquals(true,resultValue); System.assertEquals(2,visits); }
 @IsTest static void listHelperCase() { List<String> resultValue=listValue; System.assertEquals(1,resultValue.size()); System.assertEquals(2,visits); }
 @IsTest static void objectHelperCase() { Account resultValue=objectValue; System.assertEquals('x',resultValue.Name); System.assertEquals(2,visits); }
 @IsTest static void noSetterCase() { String resultValue=noSetter; System.assertEquals('x',resultValue); System.assertEquals(2,visits); }
 @IsTest static void directCase() { String resultValue=direct; System.assertEquals('x',resultValue); System.assertEquals(1,visits); }
}
