public class ClauseQuery extends ClauseHost {
    private class Filter extends Junction implements ClauseHost.Part {}
    private class ExplicitFilter extends ClauseHost.Junction implements ClauseHost.Part {}
    public static String added() { Filter f = new Filter(); f.add('red'); return String.valueOf(f.pieces.size()); }
    public static String empty() { ClauseHost.Part p = new Filter(); return String.valueOf(p.isEmpty()); }
    public static String populated() { Filter f = new Filter(); f.add('blue'); ClauseHost.Part p = f; return String.valueOf(p.isEmpty()); }
    public static String qualified() { ExplicitFilter f = new ExplicitFilter(); f.add('green'); ClauseHost.Part p = f; return String.valueOf(p.isEmpty()); }
}
