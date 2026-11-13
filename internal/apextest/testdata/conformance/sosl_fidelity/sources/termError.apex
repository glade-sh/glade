Integer qs=Limits.getSoslQueries(); try{Search.query('FIND {x} RETURNING Account(Id)');obs='ok';}catch(Exception e){obs=e.getTypeName()+'|'+e.getMessage();}obs+='|'+(Limits.getSoslQueries()-qs);
