String x=new PageReference('/apex/SpreadVf67?batch=99').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
