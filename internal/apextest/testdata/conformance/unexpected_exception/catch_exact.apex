try { throw new UnexpectedException('sample'); } catch(UnexpectedException e) { System.assert(false,'CAUGHT|'+e.getMessage()); }
