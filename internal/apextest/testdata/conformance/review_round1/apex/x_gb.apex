insert new Gb__c(Name='g1', Ck__c=true, Cu__c=1.5, Pc__c=2.5, Nm__c=3.25, Tx__c='t');
List<String> out = new List<String>();
for (String f : new List<String>{'Ck__c','Cu__c','Pc__c','Nm__c','Tx__c'}) {
  try { List<AggregateResult> rs = Database.query('SELECT ' + f + ' g, COUNT(Id) c FROM Gb__c GROUP BY ' + f); out.add(f + '=ok' + rs.size()); }
  catch (Exception e) { out.add(f + '=' + e.getTypeName() + ':' + e.getMessage().left(60)); }
}
System.debug('OBS|gb|' + String.join(out, ';'));
try { Database.query('SELECT Nm__c FROM Gb__c GROUP BY Nm__c'); } catch (System.QueryException e) { System.debug('OBS|gbq|QueryException'); } catch (Exception e) { System.debug('OBS|gbq|' + e.getTypeName()); }
try { List<AggregateResult> r2 = [SELECT Nm__c, COUNT(Id) FROM Gb__c GROUP BY Nm__c]; System.debug('OBS|gbs|ok'); } catch (Exception e) { System.debug('OBS|gbs|' + e.getTypeName()); }