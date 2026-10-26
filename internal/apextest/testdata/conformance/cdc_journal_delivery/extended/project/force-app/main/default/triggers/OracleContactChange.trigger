trigger OracleContactChange on ContactChangeEvent (after insert) {for(ContactChangeEvent event:Trigger.new) PathObservation.cdcLog(event,event.ChangeEventHeader);}
