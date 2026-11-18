@IsTest
private class Hx8Test {
 @IsTest static void observe() {
List<String> rows = new List<String>();
Long M = 4294967296L;
Hc__c p = new Hc__c(Name='P', Dec__c=1.5); insert p;
Hc__c c = new Hc__c(Name='C', Parent__c=p.Id, Dec__c=1.5); insert c;
Hc__c qp = [SELECT Id, Name, Dec__c FROM Hc__c WHERE Id = :p.Id];
Long idE = 'Id'.hashCode() ^ String.valueOf(p.Id).hashCode();
Long nmE = 'Name'.hashCode() ^ 'P'.hashCode();
Long decPad = 'Dec__c'.hashCode() ^ 4652;
Long decUnp = 'Dec__c'.hashCode() ^ 466;
Long direct = qp.hashCode();
rows.add('qpScale=' + qp.Dec__c.scale() + '|direct=padded:' + (Math.mod(Math.mod(direct - idE - nmE - decPad, M) + M, M) == 0) + ',unpadded:' + (Math.mod(Math.mod(direct - idE - nmE - decUnp, M) + M, M) == 0));
List<Hc__c> l = new List<Hc__c>{qp};
Long lh = l.hashCode();
rows.add('list=padded:' + (Math.mod(Math.mod(lh - 31 - idE - nmE - decPad, M) + M, M) == 0) + ',unpadded:' + (Math.mod(Math.mod(lh - 31 - idE - nmE - decUnp, M) + M, M) == 0));
Set<Hc__c> s = new Set<Hc__c>{qp};
Long sh = s.hashCode();
rows.add('set=padded:' + (Math.mod(Math.mod(sh - 1 - idE - nmE - decPad, M) + M, M) == 0) + ',unpadded:' + (Math.mod(Math.mod(sh - 1 - idE - nmE - decUnp, M) + M, M) == 0));
List<Decimal> dl = new List<Decimal>{qp.Dec__c}; Set<Decimal> ds = new Set<Decimal>{qp.Dec__c};
rows.add('decList=' + dl.hashCode() + '|decSet=' + ds.hashCode() + '|scale=' + qp.Dec__c.scale());
Hc__c qc = [SELECT Id, Parent__r.Name, Parent__r.Dec__c FROM Hc__c WHERE Id = :c.Id];
Hc__c nested = qc.Parent__r;
Long nIdE = 'Id'.hashCode() ^ String.valueOf(nested.Id).hashCode();
Long nPad = nIdE + nmE + decPad; Long nUnp = nIdE + nmE + decUnp;
Long nh = nested.hashCode();
rows.add('nestedDirect=padded:' + (Math.mod(Math.mod(nh - nPad, M) + M, M) == 0) + ',unpadded:' + (Math.mod(Math.mod(nh - nUnp, M) + M, M) == 0));
Long outerOwn = ('Id'.hashCode() ^ String.valueOf(c.Id).hashCode()) + ('Parent__c'.hashCode() ^ String.valueOf(p.Id).hashCode());
Long rk = 'Parent__r'.hashCode();
Long qh = qc.hashCode();
rows.add('outer=padded:' + (Math.mod(Math.mod(qh - outerOwn - (rk ^ nPad), M) + M, M) == 0) + ',unpadded:' + (Math.mod(Math.mod(qh - outerOwn - (rk ^ nUnp), M) + M, M) == 0) + ',usesNestedHash:' + (Math.mod(Math.mod(qh - outerOwn - (rk ^ nested.hashCode()), M) + M, M) == 0));
List<Hc__c> lc = new List<Hc__c>{qc};
Long lch = lc.hashCode();
rows.add('listOuter=consistent:' + (Math.mod(Math.mod(lch - 31 - qh, M) + M, M) == 0));
 System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
