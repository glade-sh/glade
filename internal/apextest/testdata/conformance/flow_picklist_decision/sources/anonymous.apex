Map<String, String> answers = MeadowDecisionProbe.observe();
System.debug('PROBE|'+JSON.serialize(answers));
System.assert(false, 'PROBE|'+JSON.serialize(answers));
