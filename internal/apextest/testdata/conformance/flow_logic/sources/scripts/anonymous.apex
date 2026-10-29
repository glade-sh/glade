Map<String, Object> answers = LogicProbe.observe();
System.debug('PROBE|'+JSON.serialize(answers));
System.assert(false, 'PROBE|'+JSON.serialize(answers));
