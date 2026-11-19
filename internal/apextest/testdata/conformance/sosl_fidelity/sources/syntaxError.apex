Integer qs=Limits.getSoslQueries(); try{Search.query('WRONG');obs='ok';}catch(Exception e){obs=e.getTypeName()+'|'+e.getMessage();}obs+='|'+(Limits.getSoslQueries()-qs);
