@IsTest private class ExternalProbeTest {
 @IsTest static void runCases() {
  Map<String,String> expected=new Map<String,String>{
   'personTypePhone'=>'SUCCESS','personTypeFirst'=>'SUCCESS','personTypeName'=>'INVALID_FIELD_FOR_INSERT_UPDATE',
   'businessFirst'=>'INVALID_FIELD_FOR_INSERT_UPDATE','businessBadEmail'=>'INVALID_EMAIL_ADDRESS',
   'businessValidEmail'=>'INVALID_FIELD_FOR_INSERT_UPDATE','businessBlankEmail'=>'SUCCESS',
   'personName'=>'INVALID_FIELD_FOR_INSERT_UPDATE','personPhone'=>'SUCCESS',
   'unmatchedFirst'=>'INVALID_FIELD_FOR_INSERT_UPDATE','unmatchedBadEmail'=>'INVALID_EMAIL_ADDRESS',
   'unmatchedValidEmail'=>'INVALID_FIELD_FOR_INSERT_UPDATE','unmatchedBlankEmail'=>'SUCCESS'};
  List<Object> rows=ExternalProbe.run(); System.assertEquals(expected.size(),rows.size());
  for(Object item:rows) {
   Map<String,Object> row=(Map<String,Object>)item; String key=(String)row.get('case');
   System.assert(expected.containsKey(key),key); System.assertEquals(expected.get(key),(String)row.get('status'),key);
   Boolean triggered=key.endsWith('Phone') || key.endsWith('ValidEmail') || key.endsWith('BlankEmail') || key=='personTypeFirst';
   System.assertEquals(triggered?1:0,((List<Boolean>)row.get('before')).size(),key);
   System.assertEquals(key.startsWith('unmatched'),(Boolean)row.get('created'),key);
  }
 }
}
