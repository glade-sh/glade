String x=new PageReference('/apex/SpreadVf67?batch=161').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
