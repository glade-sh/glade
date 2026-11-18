@IsTest private class ProbeLabelValue62 { @IsTest static void observe() { Map<String,Object> data=new Map<String,Object>();
data.put('NelValue',System.Label.get('', 'ProbeNel', 'fr')); data.put('NelExists',System.Label.translationExists('', 'ProbeNel', 'fr'));
data.put('NbspValue',System.Label.get('', 'ProbeNbsp', 'fr')); data.put('NbspExists',System.Label.translationExists('', 'ProbeNbsp', 'fr'));
data.put('FigureValue',System.Label.get('', 'ProbeFigure', 'fr')); data.put('FigureExists',System.Label.translationExists('', 'ProbeFigure', 'fr'));
data.put('NarrowValue',System.Label.get('', 'ProbeNarrow', 'fr')); data.put('NarrowExists',System.Label.translationExists('', 'ProbeNarrow', 'fr'));
data.put('PaddedNbspValue',System.Label.get('', 'ProbePaddedNbsp', 'fr')); data.put('PaddedNbspExists',System.Label.translationExists('', 'ProbePaddedNbsp', 'fr'));
data.put('PaddedTextValue',System.Label.get('', 'ProbePaddedText', 'fr')); data.put('PaddedTextExists',System.Label.translationExists('', 'ProbePaddedText', 'fr'));
System.assert(false,'CAPTURE|'+JSON.serialize(data)); } }
