public class RuntimeBoundaryProbe {
    public class ProbeException extends Exception {}
    public class SourceInfo { public Integer LineNumber; }
    public SourceInfo Source;
    public SourceInfo PropertySource { get; set; }
    public static Boolean traceEnabled = false;
    public static String trace;
    public static String helperTrace;
    public static String capture() {
        return new ProbeException().getStackTraceString();
    }
    public static void observeTrigger() {
        traceEnabled = true;
        insert new Account(Name = 'Runtime trace');
        traceEnabled = false;
        emit('trigger', JSON.serialize(new Map<String,Object>{'direct' => trace, 'helper' => helperTrace}));
    }
    public void assignNull(Integer shape) {
        if (shape == 0) this.Source.LineNumber = 7;
        if (shape == 1) this.PropertySource.LineNumber = 7;
    }
    public static void observeNull() {
        List<String> outcomes = new List<String>();
        for (Integer i = 0; i < 2; i++) {
            try { new RuntimeBoundaryProbe().assignNull(i); outcomes.add('no exception'); }
            catch (Exception e) { outcomes.add(e.getTypeName() + ':' + e.getMessage()); }
        }
        RuntimeBoundaryProbe control = new RuntimeBoundaryProbe();
        control.Source = new SourceInfo();
        control.assignNull(0);
        emit('null', JSON.serialize(new Map<String,Object>{'errors' => outcomes, 'control' => control.Source.LineNumber}));
    }
    public static void observeFlow() {
        Account a = new Account(Name = 'Runtime flow');
        insert a;
        Flow.Interview f = Flow.Interview.createInterview('RuntimeInputDefaults', new Map<String,Object>{'accountId' => a.Id});
        f.start();
        a = [SELECT Description FROM Account WHERE Id = :a.Id];
        emit('flow', JSON.serialize(a.Description));
    }
    public static void observeProfile() {
        List<Object> rows = new List<Object>();
        for (Profile p : [SELECT Name, UserType, UserLicense.Name, UserLicense.LicenseDefinitionKey FROM Profile WHERE Name IN ('Runtime Full', 'Runtime Platform', 'Runtime Chatter') ORDER BY Name]) {
            rows.add(new Map<String,Object>{'name' => p.Name, 'type' => p.UserType, 'license' => p.UserLicense.Name, 'key' => p.UserLicense.LicenseDefinitionKey});
        }
        emit('profile', JSON.serialize(rows));
    }
    public static String lastRow;
    public static void emit(String kind, String payload) {
        lastRow = 'RUNTIME_ROW|' + kind + '|' + payload;
        System.debug(lastRow);
    }
}
