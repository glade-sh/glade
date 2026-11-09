Flow.Interview f=Flow.Interview.createInterview('Q1ScalarFlow',new Map<String,Object>{'suffix'=>'yes'}); f.start();
System.assert(false,'Q1SCALAR|apex='+FeatureManagement.checkPermission('Probe_Bypass')+';flow='+String.valueOf(f.getVariableValue('result')));
