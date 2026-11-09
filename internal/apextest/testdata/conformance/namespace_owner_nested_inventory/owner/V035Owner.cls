public virtual class V035Owner {
    private class Child extends V035Owner { private Child() {} }
    private class Quiet extends V035Owner { public Quiet() {} }
    public class Pub extends V035Owner { public Pub() {} }
    public static String attempt(Type t) {
        if (t == null) { return 'null'; }
        try { Object o = t.newInstance(); return o == null ? 'nullInstance' : 'constructed'; }
        catch (Exception e) { return e.getTypeName(); }
    }
}
