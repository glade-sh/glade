public class MeadowShapeSplit {
    public static String observe() {
        MeadowWords__mdt sample = new MeadowWords__mdt(Words__c='oak,pine');
        List<String> pieces = sample.Words__c.split(',');
        return String.join(pieces, ':') + ':' + MeadowWords__mdt.getAll().size();
    }
}
