List<String> names = new List<String>{'Account','GladeMissingProbe__c','AIFeatureExtractor','AIResearchPromptResult','AIScoringStatusEvent'};
List<String> observations = new List<String>();
for (String n : names) {
 String answer = 'ok';
 try { Search.query('FIND {candidate} RETURNING '+n+'(Id)'); } catch (Exception e) { answer = e.getTypeName(); }
 observations.add(n+':'+answer);
}
System.assert(false,'OBS|'+String.join(observations,'|'));
