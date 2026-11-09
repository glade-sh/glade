public class SetHashProbe {
    public class Tok {
        public String a; public String b; public Integer extra;
        public Tok(String a, String b, Integer extra) { this.a = a; this.b = b; this.extra = extra; }
        public Boolean equals(Object o) {
            if (o == null || !(o instanceof Tok)) { return false; }
            Tok t = (Tok) o;
            return a == t.a && b == t.b;
        }
        public Integer hashCode() { return (31 * a.hashCode()) ^ b.hashCode(); }
    }
    public class Plain { public Integer n; public Plain(Integer n) { this.n = n; } public Integer hashCode() { return n; } public Boolean equals(Object o) { return o instanceof Plain && ((Plain) o).n == n; } }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        Tok x = new Tok('x', 'y', 1);
        Tok p = new Tok('p', 'q', 2);
        Tok x2 = new Tok('x', 'y', 8);
        Tok p2 = new Tok('p', 'q', 9);
        Set<Tok> s1 = new Set<Tok>{ x, p };
        Set<Tok> s2 = new Set<Tok>{ p2, x2 };
        m.put('tokHashX', x.hashCode());
        m.put('tokHashP', p.hashCode());
        m.put('twoSets1', s1.hashCode());
        m.put('twoSets2', s2.hashCode());
        m.put('twoSetsEquals', s1.equals(s2));
        m.put('twoSetsEq', s1 == s2);
        m.put('oneSet', (new Set<Tok>{ x }).hashCode());
        m.put('oneSetOther', (new Set<Tok>{ x2 }).hashCode());
        m.put('emptyTokSet', (new Set<Tok>()).hashCode());
        m.put('emptyStringSet', (new Set<String>()).hashCode());
        m.put('oneStringSet', (new Set<String>{ 'a' }).hashCode());
        m.put('stringHashA', 'a'.hashCode());
        m.put('twoStringSet', (new Set<String>{ 'a', 'b' }).hashCode());
        m.put('twoStringSetRev', (new Set<String>{ 'b', 'a' }).hashCode());
        m.put('stringHashB', 'b'.hashCode());
        m.put('intSet', (new Set<Integer>{ 1, 2, 3 }).hashCode());
        m.put('plainSet', (new Set<Plain>{ new Plain(5), new Plain(7), new Plain(11) }).hashCode());
        m.put('plainSetOne', (new Set<Plain>{ new Plain(5) }).hashCode());
        m.put('plainList2', (new List<Plain>{ new Plain(5), new Plain(7) }).hashCode());
        m.put('plainList1', (new List<Plain>{ new Plain(5) }).hashCode());
        m.put('plainList3', (new List<Plain>{ new Plain(5), new Plain(7), new Plain(11) }).hashCode());
        Map<String,Plain> mp = new Map<String,Plain>{ 'k' => new Plain(5) };
        m.put('mapOne', mp.hashCode());
        m.put('setOfSets', (new Set<Set<String>>{ new Set<String>{ 'a' } }).hashCode());
        m.put('systemHashSet', System.hashCode(s1));
        m.put('globalSet', (new Set<GlobalHashTok>{ new GlobalHashTok('x', 1), new GlobalHashTok('y', 2) }).hashCode());
        m.put('globalSetSame', (new Set<GlobalHashTok>{ new GlobalHashTok('y', 7), new GlobalHashTok('x', 8) }).hashCode());
        m.put('globalList', (new List<GlobalHashTok>{ new GlobalHashTok('x', 1), new GlobalHashTok('y', 2) }).hashCode());
        m.put('globalSystem', System.hashCode(new Set<GlobalHashTok>{ new GlobalHashTok('x', 1) }));
        m.put('systemHashList', System.hashCode(new List<Plain>{ new Plain(5), new Plain(7) }));
        m.put('emptyMap', (new Map<String,String>()).hashCode());
        m.put('twoMap', (new Map<String,String>{ 'a' => 'b', 'c' => 'd' }).hashCode());
        m.put('intMap', (new Map<Integer,Integer>{ 1 => 2, 3 => 4 }).hashCode());
        m.put('plainValueMap', (new Map<String,Plain>{ 'a' => new Plain(6), 'b' => new Plain(9) }).hashCode());
        m.put('nullSet', (new Set<String>{ null }).hashCode());
        m.put('nullList', (new List<String>{ null }).hashCode());
        m.put('emptyList', (new List<String>()).hashCode());
        m.put('stringListTwo', (new List<String>{ 'a', 'b' }).hashCode());
        m.put('systemHashStringSet', System.hashCode(new Set<String>{ 'a', 'b' }));
        m.put('systemHashMap', System.hashCode(new Map<String,String>{ 'a' => 'b' }));
        m.put('nestedList', (new List<List<Plain>>{ new List<Plain>{ new Plain(5) } }).hashCode());
        return m;
    }
}
