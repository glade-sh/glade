String r;
try { Object o = Type.forName('MissingNsV035.Thing').newInstance(); r = 'constructed'; } catch (Exception e) { r = e.getTypeName(); }
System.debug('OBSERVED|forNameNewInstance|' + r);
