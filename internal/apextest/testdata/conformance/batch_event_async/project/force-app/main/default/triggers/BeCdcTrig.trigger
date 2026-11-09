trigger BeCdcTrig on Cdc__ChangeEvent (after insert) {
    List<String> types = new List<String>();
    String note;
    for (Cdc__ChangeEvent ev : Trigger.new) {
        EventBus.ChangeEventHeader header = ev.ChangeEventHeader;
        types.add(header.changeType);
        if (ev.Name != null) { note = ev.Name; }
    }
    BeProbe.onCdc(types, note);
}
