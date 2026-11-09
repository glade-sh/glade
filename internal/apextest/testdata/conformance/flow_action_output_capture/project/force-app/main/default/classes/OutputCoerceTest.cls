@IsTest private class OutputCoerceTest {
 @IsTest static void decimalText() {
  Flow.Interview f = Flow.Interview.createInterview('OutputProbeCoercion', new Map<String,Object>{'label'=>'one'});
  f.start(); System.assertEquals('12.5', f.getVariableValue('amountResult'));
 }
}
