@isTest
private class SynthPartialCapture {
    @isTest static void capture_matrix() {
        User actor=[SELECT Id FROM User WHERE Id=:UserInfo.getUserId() WITH SYSTEM_MODE];
        System.runAs(actor) {
            System.assert(false,'SYNTH_PDML_CASES|'+JSON.serialize(SynthPartialProbe.cases()));
        }
    }
}
