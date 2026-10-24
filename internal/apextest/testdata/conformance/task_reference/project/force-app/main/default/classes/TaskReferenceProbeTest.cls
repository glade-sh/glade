@IsTest private class TaskReferenceProbeTest {
 @IsTest static void observe() { System.assert(false,'OBS|'+JSON.serialize(TaskReferenceProbe.observe())); }
}
