public class ObjectSuperShapes {
    public class Root {
        public override Integer hashCode() { return super.hashCode(); }
        public Integer parentHash() { return super.hashCode(); }
        public Boolean parentEquals(Object other) { return super.equals(other); }
    }
    public virtual class Base {
        public virtual Integer marker() { return 9; }
    }
    public class Child extends Base {
        public override Integer marker() { return super.marker() + 1; }
    }
    public static Map<String, Object> observe() {
        Root first = new Root();
        Root second = new Root();
        return new Map<String, Object>{'stable'=>first.parentHash() == first.parentHash(),
            'systemHash'=>first.parentHash() == System.hashCode(first),
            'overrideHash'=>first.hashCode() == first.parentHash(),
            'self'=>first.parentEquals(first), 'distinct'=>first.parentEquals(second),
            'null'=>first.parentEquals(null),
            'explicitSuper'=>new Child().marker()};
    }
}
