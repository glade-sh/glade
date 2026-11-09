for (String label : new List<String>{'one','negative','zero'}) {
 Flow.Interview f = Flow.Interview.createInterview('OutputProbeCoercion', new Map<String,Object>{'label'=>label}); f.start();
 String expected = label == 'negative' ? '-7.25' : label == 'zero' ? '0' : '12.5';
 System.assertEquals(expected, f.getVariableValue('amountResult'));
}
