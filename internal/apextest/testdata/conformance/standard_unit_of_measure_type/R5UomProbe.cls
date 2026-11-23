@IsTest
private class R5UomProbe {
  @IsTest static void uom() {
    List<String> r5out = new List<String>();
try {
  UnitOfMeasure u = new UnitOfMeasure(Name='Milligrams', UnitCode='mg'); insert u;
  r5out.add('R5|uom|OK Type=' + [SELECT Type FROM UnitOfMeasure WHERE Id=:u.Id].Type);
} catch (Exception e) { r5out.add('R5|uom|EXC|' + e.getMessage()); }

    System.assert(false, String.join(r5out, ';'));
  }
}
