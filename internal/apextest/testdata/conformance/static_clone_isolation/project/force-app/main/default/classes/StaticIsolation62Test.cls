@IsTest(isParallel=true)
private class StaticIsolation62Test {
    @TestSetup static void setup() { StaticIsolation62.count = 99; StaticIsolation62.trace.add('setup'); }
    @IsTest static void aWrites() {
        System.assertEquals(3, StaticIsolation62.count);
        System.assertEquals('field,block', String.join(StaticIsolation62.trace, ','));
        StaticIsolation62.count = 11;
        StaticIsolation62.trace.add('write');
        System.assertEquals(11, StaticIsolation62.count);
    }
    @IsTest static void bReads() {
        System.assertEquals(3, StaticIsolation62.count);
        System.assertEquals('field,block', String.join(StaticIsolation62.trace, ','));
    }
    @IsTest static void cInitializesOnce() {
        System.assertEquals('field,block', String.join(StaticIsolation62.trace, ','));
        Integer again = StaticIsolation62.count;
        System.assertEquals('field,block', String.join(StaticIsolation62.trace, ','));
    }
    @IsTest static void dWritesCollection() {
        System.assertEquals(2, StaticIsolation62.trace.size());
        StaticIsolation62.trace.clear();
        System.assertEquals(0, StaticIsolation62.trace.size());
    }
}
