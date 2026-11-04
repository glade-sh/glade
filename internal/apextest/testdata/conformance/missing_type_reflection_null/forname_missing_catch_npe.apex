Type t = Type.forName('NoSuchTypeQ');
try { t.newInstance(); } catch (NullPointerException e) { System.assert(false, 'caught:' + e.getTypeName() + ':' + e.getMessage()); }
