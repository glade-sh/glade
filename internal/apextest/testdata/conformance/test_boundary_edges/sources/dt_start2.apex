Test.startTest();
String r = 'first-ok';
try { Test.startTest(); r += '|second-ok'; } catch (Exception e) { r += '|caught:' + e.getTypeName() + ':' + e.getMessage(); }
Test.stopTest();
obs = r;
