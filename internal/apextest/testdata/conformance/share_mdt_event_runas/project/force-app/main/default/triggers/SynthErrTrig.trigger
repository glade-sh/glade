trigger SynthErrTrig on BatchApexErrorEvent (after insert) {
    EvtProbe.flag = Trigger.new.size();
    if (EvtProbe.mode == 'throw') {
        Set<Id> jobIds = new Set<Id>();
        for (BatchApexErrorEvent e : Trigger.new) {
            if (e.AsyncApexJobId != null) { jobIds.add(e.AsyncApexJobId); }
        }
        EvtProbe.ids = jobIds.size();
        return;
    }
    try {
        Set<Id> jobIds = new Set<Id>();
        for (BatchApexErrorEvent e : Trigger.new) {
            if (e.AsyncApexJobId != null) { jobIds.add(e.AsyncApexJobId); }
        }
        EvtProbe.ids = jobIds.size();
    } catch (Exception ex) { EvtProbe.trigErr = ex.getTypeName() + ' ' + ex.getMessage(); }
}
