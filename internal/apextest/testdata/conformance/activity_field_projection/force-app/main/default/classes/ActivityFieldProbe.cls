public class ActivityFieldProbe {
    public static Map<String,Object> observe() {
        Task taskRow = new Task(Subject='task', ProbeTag__c='alpha', ProbeState__c='Ready');
        Event eventRow = new Event(Subject='event', StartDateTime=Datetime.newInstanceGmt(2025,1,1,12,0,0), EndDateTime=Datetime.newInstanceGmt(2025,1,1,13,0,0), ProbeTag__c='beta', ProbeState__c='Ready');
        insert taskRow;
        insert eventRow;
        taskRow = [SELECT ProbeTag__c,ProbeState__c FROM Task WHERE Id=:taskRow.Id];
        eventRow = [SELECT ProbeTag__c,ProbeState__c FROM Event WHERE Id=:eventRow.Id];
        return new Map<String,Object>{
            'taskText'=>taskRow.ProbeTag__c.toUpperCase(), 'eventText'=>eventRow.ProbeTag__c.toUpperCase(),
            'taskPicklist'=>taskRow.ProbeState__c, 'eventPicklist'=>eventRow.ProbeState__c,
            'taskFieldType'=>String.valueOf(Task.ProbeTag__c.getDescribe().getType()),
            'eventFieldType'=>String.valueOf(Event.ProbeState__c.getDescribe().getType())
        };
    }
}
