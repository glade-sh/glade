@IsTest private class B6QueryIsolated67 {
 @IsTest static void capture() {
  String out='';
  for(String obj:new List<String>{'Report','Dashboard'}) {
   String query='SELECT Id FROM '+obj+' WHERE DeveloperName='+ (obj=='Report'?'\'B6_Accounts\'':'\'B6_Dashboard\'');
   try {out+=obj+'-dynamic='+Database.query(query).size()+';';}catch(Exception e){out+=obj+'-dynamic-error='+e.getTypeName()+','+e.getMessage()+';';}
   try {Database.QueryLocatorIterator it=Database.getQueryLocator(query).iterator();Integer n=0;while(it.hasNext()){it.next();n++;}out+=obj+'-locator='+n+';';}catch(Exception e){out+=obj+'-locator-error='+e.getTypeName()+','+e.getMessage()+';';}
  }
  System.assert(false,out);
 }
}
