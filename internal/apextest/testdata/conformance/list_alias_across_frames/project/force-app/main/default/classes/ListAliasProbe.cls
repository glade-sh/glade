public class ListAliasProbe {
    static List<String> cache;
    static Map<String,String> mcache;
    static Set<String> scache;
    public static List<String> get() {
        if (cache != null && cache.size() > 0) { return cache; }
        List<String> t = new List<String>{'a','b'};
        cache = t;
        return t;
    }
    public static List<String> get2() {
        if (cache == null) { cache = new List<String>{'a','b'}; }
        return cache;
    }
    public static Map<String,String> mget() {
        if (mcache == null) { mcache = new Map<String,String>{'a' => 'b'}; }
        return mcache;
    }
    public static Set<String> sget() {
        if (scache == null) { scache = new Set<String>{'a', 'b'}; }
        return scache;
    }
    static void clearViaLocal() { List<String> c = get(); c.clear(); }
    static void clearViaCall() { get().clear(); }
    static void clearStatic() { cache.clear(); }
    static void clearLocalOfStatic() { List<String> c = cache; c.clear(); }
    static void addStatic() { cache.add('z'); }
    static void addParam(List<String> p) { p.add('p'); }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        cache = null; mcache = null; scache = null;
        List<String> t1 = get(); clearViaLocal(); m.put('viaLocal', t1.size());
        cache = null;
        List<String> t2 = get(); clearViaCall(); m.put('viaCall', t2.size());
        cache = null;
        List<String> t3 = get(); clearStatic(); m.put('staticClear', t3.size());
        cache = null;
        List<String> t4 = get(); clearLocalOfStatic(); m.put('localOfStatic', t4.size());
        cache = null;
        List<String> t5 = get2(); clearLocalOfStatic(); m.put('get2', t5.size());
        cache = null;
        List<String> t6 = get(); addStatic(); m.put('addStatic', t6.size());
        cache = null;
        List<String> t7 = get(); addParam(t7); m.put('addParam', cache.size());
        cache = null;
        List<String> t8 = get(); get().add('q'); m.put('addViaCall', t8.size());
        Map<String,String> m1 = mget(); mget().clear(); m.put('mapClear', m1.size());
        Set<String> s1 = sget(); sget().clear(); m.put('setClear', s1.size());
        List<String> t9 = get2(); List<String> t10 = get2(); t9.add('w'); m.put('twoGets', t10.size());
        return m;
    }
}
