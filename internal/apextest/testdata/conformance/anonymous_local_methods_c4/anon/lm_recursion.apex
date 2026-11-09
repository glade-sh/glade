Integer fact(Integer n) { return n <= 1 ? 1 : n * fact(n - 1); }
System.debug('CAP|lm|' + fact(5));
