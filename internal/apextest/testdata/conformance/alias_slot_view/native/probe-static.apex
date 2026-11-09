// probe-static.apex (anonymous Apex)
C27bFin fin = new C27bFin();
C27bFull r = new C27bFull();
C27bSHold.s = r;
System.debug('OBS|staticPlain|' + C27bSHold.viaStatic(fin));
C27bRoot x = C27bSHold.s;
x.count = 5;
System.debug('OBS|staticAssignViaSupertypeVar|' + C27bSHold.viaStatic(fin) + '|' + C27bSHold.s.count);
C27bSHold.bump(C27bSHold.s);
System.debug('OBS|staticMutateViaSupertypeParam|' + C27bSHold.viaStatic(fin) + '|' + C27bSHold.s.count);
C27bFull r2 = new C27bFull();
C27bSHold.s = r2;
C27bRoot y = r2;
y.count = 9;
System.debug('OBS|staticAssignViaLocalAlias|' + C27bSHold.viaStatic(fin) + '|' + C27bSHold.s.count);
for (C27bRoot z : new List<C27bRoot>{ r2 }) { z.count = 11; }
System.debug('OBS|staticLoopOverSupertypeList|' + C27bSHold.viaStatic(fin) + '|' + C27bSHold.s.count);
