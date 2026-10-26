@IsTest(isParallel=true)
private class StaticIsolation67Test {
    @TestSetup static void setup() { StaticIsolation67.count = 99; StaticIsolation67.trace.add('setup'); }
    @IsTest static void aWrites() {
        System.assertEquals(3, StaticIsolation67.count);
        System.assertEquals('field,block', String.join(StaticIsolation67.trace, ','));
        StaticIsolation67.count = 11;
        StaticIsolation67.trace.add('write');
        System.assertEquals(11, StaticIsolation67.count);
    }
    @IsTest static void bReads() {
        System.assertEquals(3, StaticIsolation67.count);
        System.assertEquals('field,block', String.join(StaticIsolation67.trace, ','));
    }
    @IsTest static void cInitializesOnce() {
        System.assertEquals('field,block', String.join(StaticIsolation67.trace, ','));
        Integer again = StaticIsolation67.count;
        System.assertEquals('field,block', String.join(StaticIsolation67.trace, ','));
    }
    @IsTest static void dWritesCollection() {
        System.assertEquals(2, StaticIsolation67.trace.size());
        StaticIsolation67.trace.clear();
        System.assertEquals(0, StaticIsolation67.trace.size());
    }
}
