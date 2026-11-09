try { Test.getEventBus().deliver(); System.assert(false, 'OBSERVED|accepted'); } catch (System.TypeException e) { System.assert(false, 'OBSERVED|' + e.getTypeName()); }
