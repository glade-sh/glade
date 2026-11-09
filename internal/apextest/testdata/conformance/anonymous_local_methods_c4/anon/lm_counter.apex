Integer counter = 0;
void bump() { counter++; }
bump();
bump();
System.debug('CAP|lm|' + counter);
