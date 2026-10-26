@IsTest private class StaticAliasProbeTest {
    @IsTest static void assignment() { System.assert(false, 'OBS|assignment|' + JSON.serialize(StaticAliasProbe.observe('assignment'))); }
    @IsTest static void put() { System.assert(false, 'OBS|put|' + JSON.serialize(StaticAliasProbe.observe('put'))); }
    @IsTest static void empty() { System.assert(false, 'OBS|empty|' + JSON.serialize(StaticAliasProbe.observe('empty'))); }
    @IsTest static void rejected() { System.assert(false, 'OBS|rejected|' + JSON.serialize(StaticAliasProbe.observe('rejected'))); }
    @IsTest static void nested() { System.assert(false, 'OBS|nested|' + JSON.serialize(StaticAliasProbe.observe('nested'))); }
    @IsTest static void move() { System.assert(false, 'OBS|move|' + JSON.serialize(StaticAliasProbe.observe('move'))); }
}
