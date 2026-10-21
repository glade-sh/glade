public virtual class UnitRegressionGateway62 {
    public static UnitRegressionGateway62 mockInstance;

    public UnitRegressionGateway62() {}

    public static UnitRegressionGateway62 newInstance() {
        return mockInstance != null && Test.isRunningTest() ? mockInstance : new UnitRegressionGateway62();
    }

    public virtual String cancel() {
        return 'real';
    }
}
