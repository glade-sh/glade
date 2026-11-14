public virtual class SuperProbe {
    public virtual String value() { return 'a'; }
    public class Child extends SuperProbe {
        public override String value() { return super.
            value(); }
    }
}
