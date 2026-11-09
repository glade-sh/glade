try { Test.enableChangeDataCapture(); System.assert(false, 'OBSERVED|accepted'); } catch (System.TypeException e) { System.assert(false, 'OBSERVED|' + e.getTypeName()); }
