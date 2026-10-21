trigger HelperBoundary on Account (before insert) {private void decorate(Account row) {row.Description='helper';} decorate(Trigger.new[0]);}
