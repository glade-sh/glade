String x=new PageReference('/apex/SpreadVf68?batch=202').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
