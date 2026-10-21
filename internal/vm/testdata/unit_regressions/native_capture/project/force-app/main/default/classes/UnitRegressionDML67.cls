public virtual class UnitRegressionDML67 {
    public static UnitRegressionDML67 mockInstance;

    public UnitRegressionDML67() {}

    public static UnitRegressionDML67 newInstance() {
        return mockInstance != null && Test.isRunningTest() ? mockInstance : new UnitRegressionDML67();
    }

    public virtual List<Database.SaveResult> updateRecords(List<SObject> records) {
        return null;
    }
}
