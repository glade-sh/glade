@IsTest private class ExtProbeTest {
    @IsTest static void extProbe() { System.assertEquals('', 'OBSERVED_EXT\n' + String.join(ExtProbe.run(), '\n')); }
}
