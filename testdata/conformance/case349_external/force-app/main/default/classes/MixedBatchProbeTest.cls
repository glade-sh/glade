@IsTest private class MixedBatchProbeTest {
 @IsTest static void runCases() {
  List<Object> rows=MixedBatchProbe.run(); System.assertEquals(3,rows.size());
  for(Object item:rows) {
   Map<String,Object> row=(Map<String,Object>)item;
   System.assertEquals('exception',(String)row.get('outcome'));
   System.assertEquals(new List<String>{'INVALID_EMAIL_ADDRESS','INVALID_FIELD_FOR_INSERT_UPDATE'},(List<String>)row.get('statuses'));
   System.assertEquals(new List<Integer>{0,1},(List<Integer>)row.get('indices'));
   System.assertEquals(0,((List<Boolean>)row.get('before')).size());
  }
 }
}
