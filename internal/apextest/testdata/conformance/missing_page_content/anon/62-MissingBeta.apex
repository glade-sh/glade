try { Blob b=new PageReference('/apex/MissingBeta').getContentAsPDF(); System.debug('CAPTURE|rendered'); } catch(Exception e) { System.debug('CAPTURE|caught|'+e.getTypeName()+'|'+e.getMessage()); }
