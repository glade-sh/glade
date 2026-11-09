@IsTest public class ListPartialDmlProbeTest {
    @IsTest static void matrix() { System.assert(false, 'OBS|' + JSON.serialize(ListPartialDmlProbe.matrix())); }
    @IsTest static void ops() { System.assert(false, 'OBS|' + JSON.serialize(ListPartialDmlProbe.ops())); }
}
