public class ShareProbe {
    public static String run(String tag) {
        String out = ''; String reason = 'Synth_Reason__c';
        try {
            Synth_Doc__c d = new Synth_Doc__c(Name = 'd1');
            insert d;
            List<Synth_Doc__Share> pre = [SELECT Id, ParentId, RowCause, AccessLevel FROM Synth_Doc__Share WHERE ParentId = :d.Id];
            out += 'pre=' + pre.size() + ':' + (pre.isEmpty() ? '' : pre[0].RowCause + '/' + pre[0].AccessLevel) + ';';
            Group g = new Group(Name = 'SynthG', Type = 'Regular');
            insert g;
            insert new Synth_Doc__Share(ParentId = d.Id, UserOrGroupId = g.Id, AccessLevel = 'Read', RowCause = reason);
            List<Synth_Doc__Share> post = [SELECT Id, ParentId, RowCause, AccessLevel FROM Synth_Doc__Share WHERE RowCause = :reason];
            out += 'post=' + post.size() + ':' + post[0].RowCause + '/' + post[0].AccessLevel + ';';
            out += 'cnt=' + [SELECT COUNT() FROM Synth_Doc__Share WHERE ParentId = :d.Id AND AccessLevel = 'Read'] + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String mdt0(String tag) {
        String out = '';
        try {
            List<Synth__mdt> l = [SELECT Id, DeveloperName, MasterLabel, Label, Val__c FROM Synth__mdt ORDER BY DeveloperName];
            for (Synth__mdt m : l) out += m.DeveloperName + ',' + m.MasterLabel + ',' + m.Label + ',' + m.Val__c + ';';
            out += 'get=' + ((SObject) l[0]).get('MasterLabel') + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String mdt1(String tag) {
        String out = '';
        try {
            Synth__mdt w = [SELECT MasterLabel FROM Synth__mdt WHERE MasterLabel = 'Beta Label' LIMIT 1];
            out += 'where=' + w.MasterLabel + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String mdt2(String tag) {
        String out = '';
        try {
            out += 'ml_asc=' + names([SELECT DeveloperName FROM Synth__mdt ORDER BY MasterLabel ASC]) + ';';
            out += 'ml_desc=' + names([SELECT DeveloperName FROM Synth__mdt ORDER BY MasterLabel DESC]) + ';';
            out += 'l_asc=' + names([SELECT DeveloperName FROM Synth__mdt ORDER BY Label ASC]) + ';';
            out += 'l_desc=' + names([SELECT DeveloperName FROM Synth__mdt ORDER BY Label DESC]) + ';';
            out += 'dn_desc=' + names([SELECT DeveloperName FROM Synth__mdt ORDER BY DeveloperName DESC]) + ';';
            out += 'none=' + names([SELECT DeveloperName FROM Synth__mdt]) + ';';
            out += 'lim=' + names([SELECT DeveloperName FROM Synth__mdt ORDER BY MasterLabel LIMIT 1]) + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    static String names(List<Synth__mdt> l) { String r = ''; for (Synth__mdt m : l) r += m.DeveloperName + ','; return r; }
    public static String mdt4(String tag) {
        String out = '';
        try {
            Synth__mdt s = new Synth__mdt(MasterLabel = 'New Label', DeveloperName = 'New');
            out += 'new=' + s.MasterLabel + '/' + s.Label + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String mdt5(String tag) {
        String out = '';
        try {
            Synth__mdt g = Synth__mdt.getInstance('Alpha');
            out += 'gi=' + g.MasterLabel + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
    public static String mdt6(String tag) {
        String out = '';
        try {
            List<Synth__mdt> l2 = [SELECT MasterLabel FROM Synth__mdt WHERE DeveloperName = 'Alpha'];
            Synth__mdt x = l2[0];
            out += 'only=' + x.MasterLabel + ';';
            out += 'lbl=' + x.Label + ';';
        } catch (Exception e) { out += 'ERR ' + e.getTypeName() + ' ' + e.getMessage() + ';'; }
        return tag + '|' + out;
    }
}
