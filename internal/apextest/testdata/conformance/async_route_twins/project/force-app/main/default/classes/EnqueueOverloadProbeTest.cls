@IsTest private class EnqueueOverloadProbeTest {
@IsTest static void observeplain() { Test.startTest(); String returned=System.enqueueJob(new RouteIdentityQueue('plain')); insert new Account(Name='plainReturn',Phone=returned); Test.stopTest(); System.assert(false,'OBS|'+JSON.serialize(RouteTwinProbe.observe())); }
@IsTest static void observeinteger() { Test.startTest(); String returned=System.enqueueJob(new RouteIdentityQueue('integer'),0); insert new Account(Name='integerReturn',Phone=returned); Test.stopTest(); System.assert(false,'OBS|'+JSON.serialize(RouteTwinProbe.observe())); }
@IsTest static void observeoptions() { Test.startTest(); String returned=System.enqueueJob(new RouteIdentityQueue('options'),new AsyncOptions()); insert new Account(Name='optionsReturn',Phone=returned); Test.stopTest(); System.assert(false,'OBS|'+JSON.serialize(RouteTwinProbe.observe())); }
}
