@IsTest public class TfProbeTest {
    @IsTest static void part0() { System.assert(false, 'OBS|' + JSON.serialize(TfProbe.chunk(0, 4))); }
    @IsTest static void part1() { System.assert(false, 'OBS|' + JSON.serialize(TfProbe.chunk(1, 4))); }
    @IsTest static void part2() { System.assert(false, 'OBS|' + JSON.serialize(TfProbe.chunk(2, 4))); }
    @IsTest static void part3() { System.assert(false, 'OBS|' + JSON.serialize(TfProbe.chunk(3, 4))); }
}
