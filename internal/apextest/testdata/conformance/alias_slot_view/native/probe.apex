C27bFin fin = new C27bFin();
C27bFull r = new C27bFull();
C27bFull f = new C27bFull(r);
f.fin = fin;
System.debug('OBS|plain|' + f.viaField());

C27bHolder.cache.add(f);
C27bRoot x = r;
x.count = 5;
System.debug('OBS|assignViaSupertypeVar|' + f.viaField() + '|' + r.count);

C27bHolder.bump(r);
System.debug('OBS|mutateViaSupertypeParam|' + f.viaField() + '|' + r.count);

C27bHolder.held = f;
C27bRoot y = f.post;
y.count = 9;
System.debug('OBS|staticHolderAssign|' + C27bHolder.held.viaField() + '|' + r.count);

List<C27bFull> local = new List<C27bFull>{ f };
for (C27bRoot z : new List<C27bRoot>{ r }) { z.count = 11; }
System.debug('OBS|loopOverSupertypeList|' + local[0].viaField() + '|' + r.count);

C27bFull nofin = new C27bFull(r);
try { nofin.viaField(); System.debug('OBS|nullReceiver|none'); }
catch (Exception e) { System.debug('OBS|nullReceiver|' + e.getTypeName() + '|' + e.getMessage()); }
