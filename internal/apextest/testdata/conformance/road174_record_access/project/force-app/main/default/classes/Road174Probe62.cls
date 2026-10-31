@IsTest private class Road174Probe62 {
  @IsTest static void check() {
    for (String name : new List<String>{'Lead','Campaign','Contract','Asset','Product2','Pricebook2','Order','OpportunityLineItem','Account','Contact','Case'}) {
      Database.query('SELECT UserRecordAccess.HasReadAccess FROM ' + name + ' LIMIT 1');
    }
    for (String name : new List<String>{'Task','Event','User','Profile'}) {
      Boolean rejected = false;
      try { Database.query('SELECT UserRecordAccess.HasReadAccess FROM ' + name + ' LIMIT 1'); } catch (QueryException e) { rejected = true; }
      System.assert(rejected, name);
    }
  }
}
