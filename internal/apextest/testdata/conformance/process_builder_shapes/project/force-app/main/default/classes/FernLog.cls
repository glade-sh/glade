public class FernLog {
    public static List<String> events = new List<String>();
    public static String show(SObject r) {
        return 'S=' + r.get('Stage__c') + ',N=' + r.get('Note__c') + ',O=' + r.get('Old__c') + ',C=' + r.get('Score__c') + ',F=' + r.get('Flag__c') + ',D=' + r.get('Day__c');
    }
    public static void fire(String who) {
        String phase = (Trigger.isBefore ? 'B' : 'A') + (Trigger.isInsert ? 'I' : 'U');
        String old = Trigger.isUpdate ? '<' + Trigger.old[0].get('Note__c') + '/' + Trigger.old[0].get('Score__c') + '>' : '';
        events.add(who + '.' + phase + Trigger.size + '(' + show(Trigger.new[0]) + ')' + old);
    }
    public static void mark(String tag, List<SObject> rows) {
        events.add(tag + rows.size() + '(' + show(rows[0]) + ')');
    }
    public static String take() {
        String out = String.join(events, ' ');
        events.clear();
        return out;
    }
    public static String failure(Exception e) {
        String m = e.getMessage();
        Integer at = m.indexOf('Error ID:');
        if (at >= 0) { m = m.substring(0, at) + 'Error ID: X'; }
        return e.getTypeName() + ':' + m.replaceAll('[\\r\\n]+', ' ').replaceAll('[a-zA-Z0-9]{18}', 'ID18').replaceAll('[a-zA-Z0-9]{15}', 'ID15');
    }
}
