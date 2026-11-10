@IsTest
private class DWGetterProbe67 {
    private class DirectValue {
        public Integer visits = 0;
        public String value { get { visits++; return 'direct'; } }
    }
    private class FiniteValue {
        public Integer visits = 0;
        public String value { get { visits++; if (visits == 1) return helper(); return 'finite'; } }
        public String helper() { return value; }
    }
    private class RecursiveValue {
        public Integer visits = 0;
        public String value { get { visits++; return helper(); } }
        public String helper() { return value; }
    }
    private static String transform(Object payload) {
        return DataWeave.Script.createScript('getterprobe').execute(new Map<String,Object>{'payload'=>payload}).getValueAsString();
    }
    @IsTest static void plainControl() {
        System.assertEquals('\"plain\"', transform(new Map<String,Object>{'value'=>'plain'}));
    }
    @IsTest static void directGetter() {
        DirectValue item = new DirectValue();
        System.assertEquals('\"direct\"|visits=1', transform(item) + '|visits=' + item.visits);
    }
    @IsTest static void finiteGetter() {
        FiniteValue item = new FiniteValue();
        System.assertEquals('\"finite\"|visits=2', transform(item) + '|visits=' + item.visits);
    }
    @IsTest static void recursiveGetter() {
        RecursiveValue item = new RecursiveValue();
        System.assertEquals('OBS', transform(item) + '|visits=' + item.visits);
    }
    @IsTest static void missingScript() {
        String actual;
        try { DataWeave.Script.createScript('unavailableSyntheticScript'); actual = 'no error'; }
        catch (Exception failure) { actual = failure.getTypeName() + '|' + failure.getMessage(); }
        System.assertEquals('System.NoDataFoundException|Could not find DataWeave script unavailableSyntheticScript', actual);
    }
}
