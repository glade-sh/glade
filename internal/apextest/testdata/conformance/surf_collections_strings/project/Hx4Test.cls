@IsTest
private class Hx4Test {
 @IsTest static void observe() {
List<String> rows = new List<String>();
Long M = 4294967296L;
rows.add('acctName=' + new Account(name='A').hashCode() + '|NAME=' + new Account(NAME='A').hashCode() + '|canon=' + new Account(Name='A').hashCode());
rows.add('acct2=' + new Account(name='A', website='w').hashCode() + '|canon2=' + new Account(Name='A', Website='w').hashCode());
rows.add('hcTxt=' + new Hc__c(txt__c='A').hashCode() + '|TXT=' + new Hc__c(TXT__C='A').hashCode() + '|canon=' + new Hc__c(Txt__c='A').hashCode());
rows.add('hcNum=' + new Hc__c(num__c=1).hashCode() + '|canon=' + new Hc__c(Num__c=1).hashCode() + '|dec=' + new Hc__c(dec__c=1.5).hashCode() + '|canonDec=' + new Hc__c(Dec__c=1.5).hashCode());
Hc__c p = new Hc__c(); p.put('txt__c', 'A');
Hc__c p2 = new Hc__c(); p2.put('TXT__C', 'A');
Hc__c p3 = new Hc__c(); p3.txt__c = 'A';
rows.add('put=' + p.hashCode() + '|putUpper=' + p2.hashCode() + '|dot=' + p3.hashCode());
Hc__c e1 = new Hc__c(txt__c='A'); Hc__c e2 = new Hc__c(Txt__c='A');
rows.add('eq=' + (e1 == e2) + '|setSize=' + (new Set<Hc__c>{e1, e2}).size());
rows.add('idLower=' + new Account(id='001000000000001AAA').hashCode() + '|canon=' + new Account(Id='001000000000001AAA').hashCode());
 System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
