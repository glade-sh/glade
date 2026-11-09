@IsTest private class ShapeProbeTest {
    @IsTest static void csProbe() { System.assertEquals('', 'OBSERVED_CS\n' + String.join(CsProbe.run(), '\n')); }
    @IsTest static void cs2Probe() { System.assertEquals('', 'OBSERVED_CS2\n' + String.join(CsProbe2.run(), '\n')); }
    @IsTest static void fmProbe() { System.assertEquals('', 'OBSERVED_FM\n' + String.join(FmProbe.run(), '\n')); }
}
