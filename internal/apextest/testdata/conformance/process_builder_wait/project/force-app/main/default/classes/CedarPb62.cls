public class CedarPb62 {
    public static String process() {
        List<Cedar_Member__c> one = new List<Cedar_Member__c>{ new Cedar_Member__c(Name = 'm0', Active__c = true) };
        insert one;
        one[0].Active__c = false;
        update one;
        Integer afterOne = [SELECT COUNT() FROM Cedar_Member__c];
        List<Cedar_Member__c> five = new List<Cedar_Member__c>();
        for (Integer i = 0; i < 5; i++) { five.add(new Cedar_Member__c(Name = 'f' + i, Active__c = true)); }
        insert five;
        five[0].Active__c = false;
        five[2].Active__c = false;
        update five;
        Integer afterFive = [SELECT COUNT() FROM Cedar_Member__c];
        Cedar_Member__c off = new Cedar_Member__c(Name = 'off', Active__c = false);
        insert off;
        off.Name = 'off2';
        update off;
        Integer afterUnchanged = [SELECT COUNT() FROM Cedar_Member__c];
        return 'api=62.0 afterOne=' + afterOne + ' afterFive=' + afterFive + ' afterUnchanged=' + afterUnchanged;
    }
    public static String waits() {
        Flow.Interview fi = Flow.Interview.createInterview('Cedar_Wait_Flow', new Map<String, Object>{ 'tag' => 'w62' });
        fi.start();
        List<FlowInterview> paused = [SELECT Id, PauseLabel, InterviewLabel, InterviewStatus FROM FlowInterview];
        Integer before = [SELECT COUNT() FROM Cedar_Probe__c WHERE Case__c = 'before' AND Name = 'w62'];
        Integer after = [SELECT COUNT() FROM Cedar_Probe__c WHERE Case__c = 'after' AND Name = 'w62'];
        return 'api=62.0 interviews=' + paused.size() + (paused.isEmpty() ? '' : ' status=' + paused[0].InterviewStatus + ' pauseLabel=' + paused[0].PauseLabel + ' labelStarts=' + paused[0].InterviewLabel.startsWith('Cedar Wait Flow')) + ' before=' + before + ' after=' + after;
    }
}
