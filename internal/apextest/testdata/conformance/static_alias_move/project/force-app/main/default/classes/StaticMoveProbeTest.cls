@IsTest private class StaticMoveProbeTest {
    @IsTest static void assignment() { System.assert(false, 'OBS|assignment|' + JSON.serialize(StaticMoveProbe.observe('assignment'))); }
    @IsTest static void put() { System.assert(false, 'OBS|put|' + JSON.serialize(StaticMoveProbe.observe('put'))); }
    @IsTest static void rejected() { System.assert(false, 'OBS|rejected|' + JSON.serialize(StaticMoveProbe.observe('rejected'))); }
    @IsTest static void empty() { System.assert(false, 'OBS|empty|' + JSON.serialize(StaticMoveProbe.observe('empty'))); }
}
