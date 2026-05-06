trigger WidgetTrigger on Widget__c (before insert, after insert) {
  if (Trigger.isBefore) {
    for (Widget__c widget : Trigger.new) {
      widget.Before__c = 'before';
    }
  }
  if (Trigger.isAfter) {
    for (Widget__c widget : Trigger.new) {
      System.assertEquals(null, widget.Status__c);
      System.assertEquals(null, widget.Flow__c);
    }
  }
}
