List<String> vals = new List<String>{null,'a','A','ä','å','æ','ö','ø','z','Z','é','e','É','œ','ß','ss','中','α','Я','2','10','-','_','!','ñ','n','á','à','Å','Ä','Æ','Ö','Ø','あ','ア'};
List<Account> recs = new List<Account>();
for (Integer i=0; i<vals.size(); i++) recs.add(new Account(Name='capture '+String.valueOf(i), AccountNumber=String.valueOf(i).leftPad(3,'0'), BillingCity=vals[i]));
insert recs;
