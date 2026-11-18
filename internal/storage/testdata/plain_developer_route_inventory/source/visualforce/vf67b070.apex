String x=new PageReference('/apex/SpreadVf67?batch=70').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
