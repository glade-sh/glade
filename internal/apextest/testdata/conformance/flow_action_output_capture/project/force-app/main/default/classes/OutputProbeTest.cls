@IsTest private class OutputProbeTest {
 @IsTest static void single() {
  Flow.Interview f = Flow.Interview.createInterview('OutputProbeFlow', new Map<String,Object>{'label'=>'one', 'priorNull'=>'before', 'priorUntouched'=>'before'});
  f.start();
  System.assertEquals('one', f.getVariableValue('textResult'));
  System.assertEquals(null, f.getVariableValue('priorNull'));
  System.assertEquals(null, f.getVariableValue('priorUntouched'));
  System.assertEquals(12.5, f.getVariableValue('amountResult'));
  System.assertEquals(Date.newInstance(2026,11,3), f.getVariableValue('dayResult'));
  System.assertEquals(new List<String>{'one','two'}, f.getVariableValue('itemsResult'));
 }
}
