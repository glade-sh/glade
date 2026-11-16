String x=new PageReference('/apex/SpreadVf68?batch=158').getContent().toString(); System.assert(false,'OBS|'+x.substringBetween('OBS|','|ENDOBS')+'|ENDOBS');
