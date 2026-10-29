@IsTest private class TailExecutionProbeTest {
    private static void run(String mode) {
        Test.startTest();
        if (mode == 'callerCounters') insert new Account(Name = 'Tail observer caller');
        System.enqueueJob(new TailExecutionProbe.Work(mode));
        String stopError;
        try { Test.stopTest(); }
        catch (Exception error) { stopError = error.getTypeName() + '|' + error.getMessage(); }
        System.assert(false, 'OBS|' + JSON.serialize(new Map<String,Object>{'stopError' => stopError, 'effects' => JSON.deserializeUntyped(TailExecutionProbe.report())}));
    }
    @IsTest static void callerCounters() { run('callerCounters'); }
    @IsTest static void success() { run('success'); }
    @IsTest static void failure() { run('failure'); }
    @IsTest static void child() { run('child'); }
    @IsTest static void duplicate() { run('duplicate'); }
    @IsTest static void outside() {
        String observation;
        try { System.attachFinalizer(new TailExecutionProbe.Tail('outside')); }
        catch (Exception error) { observation = error.getTypeName() + '|' + error.getMessage(); }
        System.assert(false, 'OBS|' + JSON.serialize(observation));
    }
}
