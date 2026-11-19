trigger HdBTrg on HdB__c (after update) { HdCtl.bounce(Trigger.new[0]); }
