public class MeadowShapeLower {
    public static String observe() {
        Set<String> found = new Set<String>();
        for (MeadowClient__mdt sample : [SELECT NameText__c FROM MeadowClient__mdt WHERE NameText__c != null]) {
            found.add(sample.NameText__c.toLowerCase());
        }
        MeadowClient__mdt seed = new MeadowClient__mdt(NameText__c='GROVE');
        return seed.NameText__c.toLowerCase() + ':' + found.size();
    }
}
