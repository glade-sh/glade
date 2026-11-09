trigger C4CdcTrig on C4Cdc__ChangeEvent (after insert) {
    for (C4Cdc__ChangeEvent ev : Trigger.new) {
        EventBus.ChangeEventHeader header = ev.ChangeEventHeader;
        if (header.changeType == 'CREATE') {
            C4Probe.run('cdc', ev.Name);
        }
    }
}
