trigger BoCTrg on BoC__c (after insert, after update) { Integer n = Trigger.new.size(); }
