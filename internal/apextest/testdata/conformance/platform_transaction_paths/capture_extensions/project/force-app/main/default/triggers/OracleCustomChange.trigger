trigger OracleCustomChange on ChangeOracle__ChangeEvent (after insert) {for(ChangeOracle__ChangeEvent event:Trigger.new) PathObservation.cdcLog(event,event.ChangeEventHeader);}
