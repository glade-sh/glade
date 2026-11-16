String x=new PageReference('/apex/SpreadVf68?batch=110').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
