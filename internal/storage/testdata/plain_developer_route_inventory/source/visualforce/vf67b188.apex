String x=new PageReference('/apex/SpreadVf67?batch=188').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
