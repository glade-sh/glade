String f(Integer i) { return 'int'; }
String f(String s) { return 'str'; }
System.debug('CAP|lm|' + f(1) + f('a'));
