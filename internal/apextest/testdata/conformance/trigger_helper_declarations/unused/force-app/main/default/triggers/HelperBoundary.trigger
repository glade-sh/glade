trigger HelperBoundary on Account (before insert) {private void decorate(Account row) {row.Description='helper';} for(Account row:Trigger.new) row.Description='unused';}
