try { throw new QueryException('local'); }
catch (Exception e) { System.debug('general'); }
catch (QueryException e) { System.debug('specific'); }
