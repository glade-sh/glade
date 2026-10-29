public class ReceiverOwner {
    public static Node Instance = new Node();
    public enum Level { NONE, FULL }
    public class Node extends ReceiverParent {
        public String Text = 'PARCEL';
        public Leaf Child = new Leaf();
        public String choose(String value) { return 'text'; }
        public String choose(Integer value) { return 'number'; }
    }
    public class Leaf { public String Text = 'LEAF'; }
}
