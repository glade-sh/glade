trigger WidgetEventTrigger on Widget_Event__e (after insert) {
  for (Widget_Event__e eventRecord : Trigger.new) {
    if (eventRecord.Name__c == 'Local Event') {
      RunnerState.futureRan = RunnerState.futureRan + 1;
    }
  }
}
