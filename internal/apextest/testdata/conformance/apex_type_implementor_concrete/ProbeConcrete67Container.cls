public class ProbeConcrete67Container {
    public class NestedImplementor implements ProbeNestedContract {
        public String invoke() { return 'nested'; }
    }

    public class MultiImplementor implements ProbeNestedContract, ProbeSecondContract {
        public String invoke() { return 'multiple'; }
    }
}
