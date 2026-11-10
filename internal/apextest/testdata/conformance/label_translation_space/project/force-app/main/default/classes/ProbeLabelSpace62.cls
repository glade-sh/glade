@IsTest private class ProbeLabelSpace62 { @IsTest static void observe() { Map<String,Object> data=new Map<String,Object>();
data.put('SpaceValue',System.Label.get('', 'ProbeSpace', 'fr')); data.put('SpaceExists',System.Label.translationExists('', 'ProbeSpace', 'fr'));
data.put('EmptyValue',System.Label.get('', 'ProbeEmpty', 'fr')); data.put('EmptyExists',System.Label.translationExists('', 'ProbeEmpty', 'fr'));
data.put('MissingValue',System.Label.get('', 'ProbeMissing', 'fr')); data.put('MissingExists',System.Label.translationExists('', 'ProbeMissing', 'fr'));
data.put('FilledValue',System.Label.get('', 'ProbeFilled', 'fr')); data.put('FilledExists',System.Label.translationExists('', 'ProbeFilled', 'fr'));
System.assert(false,'CAPTURE|'+JSON.serialize(data)); } }
