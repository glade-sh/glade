Integer helper() { return 9; }
class Caller { Integer go() { return helper(); } }
System.debug('CAP|lm|' + new Caller().go());
