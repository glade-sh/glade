trigger HelperBoundary on Account (before insert) {for(Account row:Trigger.new) row.Description='ordinary';}
