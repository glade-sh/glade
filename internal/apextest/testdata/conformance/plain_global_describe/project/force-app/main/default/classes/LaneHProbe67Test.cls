@IsTest
private class LaneHProbe67Test {
    @IsTest static void keys() { System.assert(false, 'OBS|' + JSON.serialize(LaneHProbe.keysDiff('LaneHKeys67'))); }
    @IsTest static void localKeys() {
        Set<String> want = new Set<String>(new LaneHProbe.StaticResourceCalloutHelper().lines('LaneHKeys67'));
        List<String> added = new List<String>(); List<String> removed = new List<String>();
        Set<String> have = Schema.getGlobalDescribe().keySet();
        for (String k : have) if (!want.contains(k)) added.add(k);
        for (String k : want) if (!have.contains(k)) removed.add(k);
        added.sort(); removed.sort();
        System.assert(false, 'OBS|' + JSON.serialize(new Map<String, Object>{'count' => have.size(), 'notInAnon' => added, 'onlyInAnon' => removed}));
    }
    @IsTest static void nullOrder0() { System.assert(false, 'OBS|' + JSON.serialize(LaneHProbe.nullOrder(0))); }
    @IsTest static void nullOrder1() { System.assert(false, 'OBS|' + JSON.serialize(LaneHProbe.nullOrder(1))); }
    @IsTest static void nullOrder2() { System.assert(false, 'OBS|' + JSON.serialize(LaneHProbe.nullOrder(2))); }
    @IsTest static void privacyFields() { System.assert(false, 'OBS|' + JSON.serialize(LaneHProbe.privacy('fields'))); }
    @IsTest static void taskWhat() { System.assert(false, 'OBS|' + JSON.serialize(LaneHProbe.privacy('Task.WhatId'))); }
    @IsTest static void eventWhat() { System.assert(false, 'OBS|' + JSON.serialize(LaneHProbe.privacy('Event.WhatId'))); }
    @IsTest static void picks() { System.assert(false, 'OBS|' + JSON.serialize(LaneHProbe.picks())); }
}
