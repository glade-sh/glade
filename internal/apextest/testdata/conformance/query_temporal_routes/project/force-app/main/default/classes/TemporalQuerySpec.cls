@IsTest private class TemporalQuerySpec {
    @IsTest static void observeDirect() { System.assert(false,'TEMPORAL|direct|'+JSON.serialize(TemporalQueryProbe.run('direct'))); }
    @IsTest static void observeTrigger() { System.assert(false,'TEMPORAL|trigger|'+JSON.serialize(TemporalQueryProbe.run('trigger'))); }
    @IsTest static void observeFlow() { System.assert(false,'TEMPORAL|flow|'+JSON.serialize(TemporalQueryProbe.run('flow'))); }
    @IsTest static void observeTriggerFlow() { System.assert(false,'TEMPORAL|triggerFlow|'+JSON.serialize(TemporalQueryProbe.run('triggerFlow'))); }
}
