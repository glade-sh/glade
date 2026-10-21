public class ProbeNestedContainer {
    public class NestedImplementor implements ProbeNestedContract {}
    public class NestedControl {}
    public class MultiImplementor implements ProbeNestedContract, ProbeSecondContract {
        public String invoke() { return 'multiple'; }
    }
    private class PrivateImplementor implements ProbeNestedContract {}
}
