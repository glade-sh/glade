try { System.Label.get('', 'AbsentLabel', 'fr'); System.debug('CAPTURE|unexpected success'); }
catch (Exception e) { System.debug('CAPTURE|' + e.getTypeName() + '|' + e.getMessage()); }
