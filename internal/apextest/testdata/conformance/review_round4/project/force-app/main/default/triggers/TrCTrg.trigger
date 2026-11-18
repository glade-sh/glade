trigger TrCTrg on TrC__c (after insert, after update) { Integer n = Trigger.new.size(); }
