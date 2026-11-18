trigger HdATrg on HdA__c (after update) { HdCtl.bounce(Trigger.new[0]); }
