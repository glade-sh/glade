public class RuntimeMissingInputCases implements Schedulable {
    public void execute(SchedulableContext context) {}
    public static void apply() {}
    public static Map<String, Object> observe() {
        Map<String, Object> result = new Map<String, Object>();
        try { System.schedule('Missing schedule boundary', null, new RuntimeMissingInputCases()); result.put('nullCron', 'allowed'); }
        catch (Exception e) { result.put('nullCron', e.getTypeName() + '|' + e.getMessage()); }
        String job = System.schedule('Future schedule boundary', '0 0 0 1 1 ? 2099', new RuntimeMissingInputCases());
        result.put('validCron', job != null);
        return result;
    }
}
