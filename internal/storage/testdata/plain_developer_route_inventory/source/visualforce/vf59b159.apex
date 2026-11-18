String x=new PageReference('/apex/SpreadVf59?batch=159').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
