public with sharing class SpreadGapVf65 {
 public String getObserved() {
  List<String> names=new List<String>{'Account','GladeMissingProbe__c','AIFeatureExtractor','AIResearchPromptResult','AIScoringStatusEvent'};
  List<String> observations=new List<String>();
  for(String n:names){String ty='false'; String q='ok'; try{ty=String.valueOf(Type.forName(n)!=null);}catch(Exception e){ty=e.getTypeName();} try{Database.query('SELECT Id FROM '+n+' LIMIT 1');}catch(Exception e){q=e.getTypeName();} observations.add(n+':'+ty+':'+q);}
  return 'OBS|'+String.join(observations,'|')+'|ENDOBS';
 }
}
