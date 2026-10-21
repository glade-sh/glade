public virtual class UnitRegressionGateway67 {
    public static UnitRegressionGateway67 mockInstance;

    public UnitRegressionGateway67() {}

    public static UnitRegressionGateway67 newInstance() {
        return mockInstance != null && Test.isRunningTest() ? mockInstance : new UnitRegressionGateway67();
    }

    public virtual String cancel() {
        return 'real';
    }
}
