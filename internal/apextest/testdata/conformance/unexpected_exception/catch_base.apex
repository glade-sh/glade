try { throw new UnexpectedException('sample'); } catch(Exception e) { System.assert(false,'CAUGHT|'+e.getMessage()); }
