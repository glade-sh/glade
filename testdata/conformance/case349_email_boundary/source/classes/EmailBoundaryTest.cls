@IsTest private class EmailBoundaryTest {
 @IsTest static void capture() {
  System.assertEquals('INVALID_EMAIL_ADDRESS|0|none', EmailBoundaryProbe.capture('false'));
  System.assertEquals('INVALID_EMAIL_ADDRESS|0|none', EmailBoundaryProbe.capture('x'));
  System.assertEquals('INVALID_EMAIL_ADDRESS|0|none', EmailBoundaryProbe.capture('x@y'));
  System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE|1|false', EmailBoundaryProbe.capture('synthetic@example.invalid'));
  System.assertEquals('SUCCESS|1|false', EmailBoundaryProbe.capture('   '));
 }
}
