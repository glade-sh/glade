class Box { Integer v; }
Box mk(Integer v) { Box b = new Box(); b.v = v; return b; }
System.debug('CAP|lm|' + mk(7).v);
