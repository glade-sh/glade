@IsTest
private class ShareProbeTest {
    @IsTest static void share() { System.assert(false, 'OUT:' + ShareProbe.run('t')); }
    @IsTest static void mdt0() { System.assert(false, 'OUT:' + ShareProbe.mdt0('t')); }
    @IsTest static void mdt1() { System.assert(false, 'OUT:' + ShareProbe.mdt1('t')); }
    @IsTest static void mdt2() { System.assert(false, 'OUT:' + ShareProbe.mdt2('t')); }
    @IsTest static void mdt4() { System.assert(false, 'OUT:' + ShareProbe.mdt4('t')); }
    @IsTest static void mdt5() { System.assert(false, 'OUT:' + ShareProbe.mdt5('t')); }
    @IsTest static void mdt6() { System.assert(false, 'OUT:' + ShareProbe.mdt6('t')); }
}
