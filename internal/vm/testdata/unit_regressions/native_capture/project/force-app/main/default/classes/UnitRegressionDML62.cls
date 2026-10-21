public virtual class UnitRegressionDML62 {
    public static UnitRegressionDML62 mockInstance;

    public UnitRegressionDML62() {}

    public static UnitRegressionDML62 newInstance() {
        return mockInstance != null && Test.isRunningTest() ? mockInstance : new UnitRegressionDML62();
    }

    public virtual List<Database.SaveResult> updateRecords(List<SObject> records) {
        return null;
    }
}
