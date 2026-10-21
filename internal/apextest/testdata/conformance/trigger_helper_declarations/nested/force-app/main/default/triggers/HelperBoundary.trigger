trigger HelperBoundary on Account (before insert) {if(Trigger.isBefore) {for(Account row:Trigger.new) row.Description='nested';}}
