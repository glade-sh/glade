Account b = new Account(Name='Capture Collection Seed B'); insert b;
Flow.Interview f = Flow.Interview.createInterview('Capture_Filter_NotIn', new Map<String,Object>{'ids'=>new List<String>{String.valueOf(b.Id)}});
f.start();
System.debug('CAPTURE|NotIn|single matching record|'+String.valueOf(f.getVariableValue('outcome')));
