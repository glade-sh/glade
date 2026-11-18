@IsTest private class ProbeLabelMore62 { @IsTest static void observe() { Map<String,Object> data=new Map<String,Object>();
data.put('TabValue',System.Label.get('', 'ProbeTab', 'fr')); data.put('TabExists',System.Label.translationExists('', 'ProbeTab', 'fr'));
data.put('NewlineValue',System.Label.get('', 'ProbeNewline', 'fr')); data.put('NewlineExists',System.Label.translationExists('', 'ProbeNewline', 'fr'));
data.put('NbspValue',System.Label.get('', 'ProbeNbsp', 'fr')); data.put('NbspExists',System.Label.translationExists('', 'ProbeNbsp', 'fr'));
data.put('EmspaceValue',System.Label.get('', 'ProbeEmspace', 'fr')); data.put('EmspaceExists',System.Label.translationExists('', 'ProbeEmspace', 'fr'));
System.assert(false,'CAPTURE|'+JSON.serialize(data)); } }
