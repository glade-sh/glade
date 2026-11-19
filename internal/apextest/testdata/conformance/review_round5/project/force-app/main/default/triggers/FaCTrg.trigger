trigger FaCTrg on FaC__c (before insert, before update, before delete) { Integer n = Trigger.isDelete ? Trigger.old.size() : Trigger.new.size(); }
