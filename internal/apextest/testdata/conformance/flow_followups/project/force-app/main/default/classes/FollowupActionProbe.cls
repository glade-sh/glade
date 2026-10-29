public class FollowupActionProbe {
    public static Map<String,Object> one(List<String> modes) {
        FlowCoreAOAddDays.sizes = new List<Integer>();
        Map<String,Object> out = new Map<String,Object>();
        List<FlowCoreAORecord__c> subjects = new List<FlowCoreAORecord__c>();
        for (Integer i=0; i<modes.size(); i++) subjects.add(new FlowCoreAORecord__c(Name='subject'+i, Unit__c=modes[i], Start__c=Datetime.newInstanceGmt(2031,3,10+i,12,0,0)));
        try {
            insert subjects;
            List<String> dates = new List<String>();
            List<String> times = new List<String>();
            for (FlowCoreAORecord__c r : [SELECT Name,DueDate__c,DueDateTime__c FROM FlowCoreAORecord__c WHERE Id IN :subjects ORDER BY Name]) {
                dates.add(r.DueDate__c == null ? null : String.valueOf(r.DueDate__c));
                times.add(r.DueDateTime__c == null ? null : r.DueDateTime__c.formatGmt('yyyy-MM-dd HH:mm:ss'));
            }
            out.put('success',true); out.put('dates',dates); out.put('times',times);
        } catch (DmlException e) {
            out.put('success',false); out.put('status',e.getDmlStatusCode(0));
            out.put('fault',e.getDmlMessage(0).contains('synthetic action fault'));
        }
        out.put('sizes', FlowCoreAOAddDays.sizes);
        return out;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> out = new Map<String,Object>();
        out.put('AO01',one(new List<String>{'empty'}));
        out.put('AO02',one(new List<String>{'null-element'}));
        out.put('AO03',one(new List<String>{'multiple'}));
        out.put('AO04',one(new List<String>{'value','value','value'}));
        out.put('AO05',one(new List<String>{'value'}));
        out.put('AO06',one(new List<String>{'null-list'}));
        out.put('AO07',one(new List<String>{'empty','empty','empty'}));
        out.put('AO08',one(new List<String>{'value','null-element','value'}));
        out.put('AO09',one(new List<String>{'multiple','multiple','multiple'}));
        out.put('AO10',one(new List<String>()));
        out.put('AO11',one(new List<String>{'throw'}));
        out.put('AO12',one(new List<String>{'value','throw','value'}));
        out.put('AO13',one(new List<String>{'value','empty','value'}));
        out.put('AO14',one(new List<String>{'null-list','null-list','null-list'}));
        return out;
    }
}
