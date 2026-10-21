global abstract class BuilderBase {
    private List<String> observers;
    global BuilderBase() { observers = new List<String>(); }
    protected List<String> insertAll(Integer n) {
        List<String> out = new List<String>();
        for (String o : observers) { out.add(o); }
        out.add('pkg');
        return out;
    }
    protected String protectedToken() { return 'pkg-protected'; }
    private String privateToken() { return 'pkg-private'; }
    global String globalToken() { return 'pkg-global'; }
    global String privateProof() { return privateToken(); }
    global static String staticToken() { return 'pkg-static'; }
}
