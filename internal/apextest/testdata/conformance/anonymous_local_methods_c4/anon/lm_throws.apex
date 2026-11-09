Integer boom() { throw new IllegalArgumentException('bad'); }
try { boom(); } catch (IllegalArgumentException e) { System.debug('CAP|lm|caught ' + e.getMessage()); }
