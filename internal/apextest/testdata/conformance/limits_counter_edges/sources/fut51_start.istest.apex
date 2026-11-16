Test.startTest();
for (Integer i = 0; i < 51; i++) { EdgeFut__A__.noop(); }
obs = Limits.getFutureCalls() + '|' + Limits.getLimitFutureCalls();
Test.stopTest();
