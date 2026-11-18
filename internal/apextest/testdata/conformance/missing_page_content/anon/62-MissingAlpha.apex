try { Blob b=new PageReference('/apex/MissingAlpha').getContentAsPDF(); System.debug('CAPTURE|rendered'); } catch(Exception e) { System.debug('CAPTURE|caught|'+e.getTypeName()+'|'+e.getMessage()); }
