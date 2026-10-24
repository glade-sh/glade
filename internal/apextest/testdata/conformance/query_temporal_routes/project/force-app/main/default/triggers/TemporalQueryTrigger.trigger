trigger TemporalQueryTrigger on Account (after insert) {
    if(TemporalQueryProbe.entry == 'trigger') {
        TemporalQueryProbe.observed = TemporalQueryProbe.observe();
        Integer directCount=0;
        for(Contact match : [SELECT Id FROM Contact WHERE LastName='Temporal probe' AND ObservedAt__c > 2020-01-01T08:00:00Z]) { directCount++; }
        TemporalQueryProbe.observed.put('trigger/literal',directCount);
    } else if(TemporalQueryProbe.entry == 'triggerFlow') {
        Flow.Interview interview=Flow.Interview.createInterview('TemporalQueryParent',new Map<String,Object>());
        interview.start();
    }
}
