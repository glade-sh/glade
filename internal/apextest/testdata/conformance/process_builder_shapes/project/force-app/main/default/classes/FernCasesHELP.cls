public class FernCasesHELP {
    public static String rows(List<SObject> rs) {
        List<String> o = new List<String>();
        for (SObject r : rs) { o.add(FernLog.show(r)); }
        return String.join(o, ' ; ');
    }
}
