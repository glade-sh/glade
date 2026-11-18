try { Blob b=new PageReference('/apex/MissingGamma').getContent(); System.debug('CAPTURE|rendered'); } catch(Exception e) { System.debug('CAPTURE|caught|'+e.getTypeName()+'|'+e.getMessage()); }
