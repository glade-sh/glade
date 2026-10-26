public class ScopeRefreshProbe {
    public class Basket {
        public List<List<Integer>> rows = new List<List<Integer>>();
        public void fill() {
            for (Integer i=0; i<32; i++) rows.add(new List<Integer>{i});
        }
        public void append() { rows[0].add(100); rows[31].add(200); }
    }
    public class Holder {
        public List<Integer> values;
        public Holder(List<Integer> values) { this.values=values; }
        public void append() { values.add(300); }
    }
    public static Map<String,Object> observe() {
        Basket basket = new Basket(); basket.fill(); basket.append();
        List<Integer> shared = basket.rows[0];
        Holder holder = new Holder(shared); holder.append();
        return new Map<String,Object>{'size'=>basket.rows.size(), 'first'=>basket.rows[0], 'last'=>basket.rows[31], 'shared'=>shared};
    }
}
