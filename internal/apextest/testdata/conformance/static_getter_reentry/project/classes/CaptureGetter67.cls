@IsTest private class CaptureGetter67 {
 static Integer visits=0;
 static String value { get { visits++; if (visits==1) { String nestedValue=helper(); } return 'done'; } set; }
 static String helper() { return value; }
 @IsTest static void finiteReentry() { String resultValue=value; System.assertEquals('done',resultValue); System.assertEquals(2,visits); }
}
