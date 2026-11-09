class Pt { Integer x = 4; }
Integer px(Pt p) { return p.x; }
System.debug('CAP|lm|' + px(new Pt()));
