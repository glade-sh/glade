@IsTest private class CpuProbe67Test {
 @IsTest static void observe() {
  Integer before = Limits.getCpuTime(); Integer sum = 0;
  for (Integer i=0;i<100000;i++) { sum += Math.mod(i,97); }
  Integer after = Limits.getCpuTime();
  System.assert(before >= 0); System.assert(after >= before); System.assert(after > before);
  System.assertEquals(10000, Limits.getLimitCpuTime());
  Test.startTest(); Integer fresh = Limits.getCpuTime();
  System.assert(fresh <= after); System.assertEquals(10000, Limits.getLimitCpuTime());
  System.enqueueJob(new CpuProbe67.Child()); Test.stopTest();
  System.assert(Limits.getCpuTime() >= after);
 }
}
