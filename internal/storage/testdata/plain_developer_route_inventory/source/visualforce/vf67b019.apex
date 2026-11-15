String x=new PageReference('/apex/SpreadVf67?batch=19').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
