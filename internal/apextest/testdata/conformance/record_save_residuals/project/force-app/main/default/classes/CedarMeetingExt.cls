public with sharing class CedarMeetingExt {
    public List<Event> meetings { get; set; }
    public List<Task> goals { get; set; }
    public CedarMeetingExt(ApexPages.StandardController sc) {
        meetings = [SELECT Id, Cedar_Account__r.Name FROM Event LIMIT 5];
        goals = [SELECT Id, Cedar_Account__r.Name FROM Task LIMIT 5];
    }
}
