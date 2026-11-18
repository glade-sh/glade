public class CovInner {
    public class NestedBox {
        public Integer value;

        public NestedBox(Integer v) {
            value = v;
        }

        public Integer doubled() {
            return value * 2;
        }
    }

    public class InstanceBox {
        public String label;

        public InstanceBox(String l) {
            label = l;
        }

        public String shout() {
            return label.toUpperCase();
        }
    }

    public static Integer useNested(Integer v) {
        NestedBox box = new NestedBox(v);
        return box.doubled();
    }

    public String useInstance(String l) {
        InstanceBox box = new InstanceBox(l);
        return box.shout();
    }
}
