System.debug('API68PLUMB|boundary|'+String.valueOf(Date.valueOf('2026-11-03')));
try { JSON.deserialize('null', Date.class); System.debug('API68PLUMB|date-null|SUCCESS'); } catch (Exception e) { System.debug('API68PLUMB|date-null|'+e.getTypeName()+'|'+e.getMessage()); }
try { JSON.deserialize('"2026-11-03"', Date.class); System.debug('API68PLUMB|date-valid|SUCCESS'); } catch (Exception e) { System.debug('API68PLUMB|date-valid|'+e.getTypeName()+'|'+e.getMessage()); }
