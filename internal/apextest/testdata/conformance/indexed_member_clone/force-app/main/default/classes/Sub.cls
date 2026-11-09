public class Sub extends HolderA {
    public Integer go() {
        List<String> c = items[0].parts.clone();
        List<String> d = this.items[0].parts.clone();
        Item q = items[0];
        List<String> e = q.parts.clone();
        for (Item i : items) { List<String> f = i.parts.clone(); }
        return c.size() + d.size() + e.size();
    }
}
