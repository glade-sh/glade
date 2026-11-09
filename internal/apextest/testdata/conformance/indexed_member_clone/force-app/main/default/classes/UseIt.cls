public class UseIt {
    public static Integer run() {
        HolderA h = new HolderA();
        HolderA.Item it = new HolderA.Item();
        it.parts.add('a'); it.parts.add('b');
        h.items.add(it);
        HolderB b = new HolderB();
        b.items.add(new HolderB.Item());
        Integer n = 0;
        for (Integer i = 0; i < h.items.size(); i++) {
            List<String> c = h.items[i].parts.clone();
            n += c.size() + h.items[0].parts.size();
            for (String s : h.items[i].parts) { n += s.length(); }
        }
        return n + b.items[0].parts.length();
    }
}
