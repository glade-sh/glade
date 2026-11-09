public class CedarProbe67 {
    public static String run() {
        return 'api=67.0 A62=' + CedarProbe.afterSave('Cedar_Report62__c') + ' A67=' + CedarProbe.afterSave('Cedar_Report__c') + ' C62=' + CedarProbe.create('CedarCreate62') + ' C67=' + CedarProbe.create('CedarCreate67');
    }
    public static String groupedRun() { CedarProbe.seedGrouped(); return 'api=67.0 G=' + groupedLocal(); }
    static String groupedLocal() {
        String out = '';
                return out + 'dynamic=' + CedarProbe.grouped();
    }
}
