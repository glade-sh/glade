public virtual class ClauseHost {
    public interface Part { Boolean isEmpty(); }
    public virtual class Junction {
        public List<String> pieces = new List<String>();
        public Boolean isEmpty() { return pieces.isEmpty(); }
        public void add(String value) { pieces.add(value); }
    }
}
