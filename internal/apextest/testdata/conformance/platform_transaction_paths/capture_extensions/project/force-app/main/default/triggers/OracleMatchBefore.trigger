trigger OracleMatchBefore on MatchOracle__c (before insert) {for(MatchOracle__c row:Trigger.new) if(row.Mode__c=='before') row.Key__c='same-key';}
