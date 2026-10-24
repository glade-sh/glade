trigger OracleAccountChange on AccountChangeEvent (after insert) {for(AccountChangeEvent event:Trigger.new) PathObservation.cdcLog(event,event.ChangeEventHeader);}
