try { Pattern.matches(null, 'x'); System.debug('P01|NO_EXCEPTION'); } catch(Exception e) { System.debug('P01|' + e.getTypeName() + '|' + e.getMessage()); }
try { Pattern.matches('[0-9]+', null); System.debug('P02|NO_EXCEPTION'); } catch(Exception e) { System.debug('P02|' + e.getTypeName() + '|' + e.getMessage()); }
