@IsTest
private class Hs1HashTest {
 @IsTest static void observe() {
List<String> rows = new List<String>();
rows.add('strA=' + 'A'.hashCode());
rows.add('acctEmpty=' + new Account().hashCode());
rows.add('acctEmpty2=' + new Account().hashCode());
rows.add('contEmpty=' + new Contact().hashCode());
rows.add('oppEmpty=' + new Opportunity().hashCode());
rows.add('hcEmpty=' + new Hc__c().hashCode());
rows.add('acctNameA=' + new Account(Name='A').hashCode());
rows.add('acctNameA2=' + new Account(Name='A').hashCode());
rows.add('acctNameB=' + new Account(Name='B').hashCode());
rows.add('acctNameAA=' + new Account(Name='AA').hashCode());
rows.add('acctName1=' + new Account(Name='1').hashCode());
rows.add('acctNameNull=' + new Account(Name=null).hashCode());
rows.add('acctWebA=' + new Account(Website='A').hashCode());
rows.add('acctEmp1=' + new Account(NumberOfEmployees=1).hashCode());
rows.add('acctEmp2=' + new Account(NumberOfEmployees=2).hashCode());
rows.add('acctRev1=' + new Account(AnnualRevenue=1).hashCode());
rows.add('acctNameA_Web_w=' + new Account(Name='A', Website='w').hashCode());
rows.add('acctWeb_w_NameA=' + new Account(Website='w', Name='A').hashCode());
rows.add('acctNameA_Emp1=' + new Account(Name='A', NumberOfEmployees=1).hashCode());
rows.add('acctEmp1_NameA=' + new Account(NumberOfEmployees=1, Name='A').hashCode());
rows.add('acctBefore_w=' + new Account(Name='Before', Website='w').hashCode());
rows.add('acctAfter_w=' + new Account(Name='After', Website='w').hashCode());
Account p1 = new Account(); p1.Name = 'A'; p1.Website = 'w';
Account p2 = new Account(); p2.Website = 'w'; p2.Name = 'A';
rows.add('putOrder1=' + p1.hashCode() + '|putOrder2=' + p2.hashCode());
Account p3 = new Account(); p3.put('Name', 'A');
rows.add('putName=' + p3.hashCode());
rows.add('contLastA=' + new Contact(LastName='A').hashCode());
rows.add('hcNameA=' + new Hc__c(Name='A').hashCode());
rows.add('hcTxtA=' + new Hc__c(Txt__c='A').hashCode());
rows.add('hcNum1=' + new Hc__c(Num__c=1).hashCode());
rows.add('hcNameA_Num1=' + new Hc__c(Name='A', Num__c=1).hashCode());
rows.add('idOnly1=' + new Account(Id='001000000000001AAA').hashCode());
rows.add('idOnly2=' + new Account(Id='001000000000002AAA').hashCode());
rows.add('idOnly1b=' + new Account(Id='001000000000001AAA').hashCode());
rows.add('idNameA=' + new Account(Id='001000000000001AAA', Name='A').hashCode());
rows.add('cidOnly1=' + new Contact(Id='003000000000001AAA').hashCode());
 System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
