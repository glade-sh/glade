@IsTest private class LocaleCapture67 {
static String probe(String loc) {
Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
String un='probe'+String.valueOf(Math.abs(Crypto.getRandomInteger()))+'@example.invalid';
User u=new User(Alias='probe',Email=un,EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey=loc,ProfileId=p.Id,TimeZoneSidKey='GMT',Username=un); insert u;
String answer; System.runAs(u) {
List<String> vals = new List<String>{null,'a','A','ä','å','æ','ö','ø','z','Z','é','e','É','œ','ß','ss','中','α','Я','2','10','-','_','!','ñ','n','á','à','Å','Ä','Æ','Ö','Ø','あ','ア'};
List<Account> recs = new List<Account>();
for (Integer i=0; i<vals.size(); i++) recs.add(new Account(Name='capture '+String.valueOf(i), AccountNumber=String.valueOf(i).leftPad(3,'0'), BillingCity=vals[i]));
insert recs;

List<String> labels = new List<String>();
for (Account a : [SELECT AccountNumber FROM Account WHERE Name LIKE 'capture %' ORDER BY BillingCity ASC NULLS FIRST, AccountNumber ASC]) labels.add(a.AccountNumber);
answer=UserInfo.getLocale()+':'+String.join(labels,',');

} return answer;
}
@IsTest static void t_de_DE() { System.assertEquals('CAPTURE', probe('de_DE')); }
@IsTest static void t_sv_SE() { System.assertEquals('CAPTURE', probe('sv_SE')); }
@IsTest static void t_ja_JP() { System.assertEquals('CAPTURE', probe('ja_JP')); }
@IsTest static void t_fr_FR() { System.assertEquals('CAPTURE', probe('fr_FR')); }
@IsTest static void t_es_ES() { System.assertEquals('CAPTURE', probe('es_ES')); }
@IsTest static void t_en_US() { System.assertEquals('CAPTURE', probe('en_US')); }
}
