Blob content=new PageReference('/apex/ArithmeticScreenless').getContent();
Map<String,Object> rows=new Map<String,Object>();
for(Account a:[SELECT Name,AnnualRevenue FROM Account WHERE Name LIKE 'api62/ArithmeticScreenless%']) { rows.put(a.Name.substring(6),a.AnnualRevenue); }
System.debug(LoggingLevel.ERROR,'PROBE|'+JSON.serialize(rows));
System.debug(LoggingLevel.ERROR,'RENDER_BYTES|'+content.size());
