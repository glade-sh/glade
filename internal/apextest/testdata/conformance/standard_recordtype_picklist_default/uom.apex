try {
  UnitOfMeasure u = new UnitOfMeasure(Name='Milligrams', UnitCode='mg'); insert u;
  System.debug('R5|uom|OK Type=' + [SELECT Type FROM UnitOfMeasure WHERE Id=:u.Id].Type);
} catch (Exception e) { System.debug('R5|uom|EXC|' + e.getMessage()); }
