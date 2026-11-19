List<String> rows = new List<String>();
Long M = 4294967296L;
Account pa = new Account(Name='P');
Contact cc = new Contact(LastName='A');
Integer hNo = cc.hashCode();
String ctorErr = 'ok';
try { cc.Account = pa; } catch (Exception e) { ctorErr = e.getTypeName() + '|' + e.getMessage(); }
rows.add('parentCtor=' + ctorErr + '|with=' + cc.hashCode() + '|without=' + hNo + '|pa=' + pa.hashCode() + '|relKey=' + 'Account'.hashCode());
Contact cc2 = new Contact(LastName='A', Account=new Account(Name='P'));
rows.add('parentInline=' + cc2.hashCode());
Account a = new Account(Name='RelP'); insert a;
Account a0 = new Account(Name='RelZ'); insert a0;
Account a1 = new Account(Name='RelO'); insert a1;
Account a3 = new Account(Name='RelT'); insert a3;
Contact c1 = new Contact(LastName='K1', AccountId=a.Id); Contact c2 = new Contact(LastName='K2', AccountId=a.Id); Contact c3 = new Contact(LastName='K3');
Contact o1 = new Contact(LastName='O1', AccountId=a1.Id);
Contact t1 = new Contact(LastName='T1', AccountId=a3.Id); Contact t2 = new Contact(LastName='T2', AccountId=a3.Id); Contact t3 = new Contact(LastName='T3', AccountId=a3.Id);
insert new List<Contact>{c1, c2, c3, o1, t1, t2, t3};
Long ck = 'Contacts'.hashCode();
for (Account x : new List<Account>{a0, a1, a, a3}) {
 Account base = [SELECT Id, Name FROM Account WHERE Id = :x.Id];
 Account withKids = [SELECT Id, Name, (SELECT Id, LastName FROM Contacts ORDER BY LastName) FROM Account WHERE Id = :x.Id];
 Account withKids2 = [SELECT Id, Name, (SELECT LastName FROM Contacts ORDER BY LastName) FROM Account WHERE Id = :x.Id];
 Long idE = 'Id'.hashCode() ^ String.valueOf(x.Id).hashCode();
 Long nmE = 'Name'.hashCode() ^ x.Name.hashCode();
 Long d = Math.mod(Math.mod((Long)withKids.hashCode() - (Long)base.hashCode(), M) + M, M);
 Long d2 = Math.mod(Math.mod((Long)withKids2.hashCode() - (Long)base.hashCode(), M) + M, M);
 Long cku = Math.mod(Math.mod(ck, M) + M, M);
 rows.add('kids[' + withKids.Contacts.size() + ']=baseOk:' + (Math.mod(Math.mod((Long)base.hashCode() - idE - nmE, M) + M, M) == 0) + '|diffXorKey=' + (d ^ cku) + '|diff2XorKey=' + (d2 ^ cku) + '|eqBase=' + (base == withKids) + '|sameHash=' + (base.hashCode() == withKids.hashCode()));
}
Contact cq0 = [SELECT Id, LastName FROM Contact WHERE Id = :c1.Id];
Contact cq1 = [SELECT Id, LastName, Account.Name FROM Contact WHERE Id = :c1.Id];
Long cIdE = 'AccountId'.hashCode() ^ String.valueOf(a.Id).hashCode();
Long candP = 'Account'.hashCode() ^ cq1.Account.hashCode();
Long dP = Math.mod(Math.mod((Long)cq1.hashCode() - (Long)cq0.hashCode() - candP - cIdE, M) + M, M);
Long nestedIdE = 'Id'.hashCode() ^ String.valueOf(cq1.Account.Id).hashCode();
Long nestedNm = 'Name'.hashCode() ^ cq1.Account.Name.hashCode();
rows.add('parentQ=' + (dP == 0) + '|acctName=' + cq1.Account.Name + '|nested=' + (Math.mod(Math.mod((Long)cq1.Account.hashCode() - nestedIdE - nestedNm, M) + M, M) == 0));
Contact cn0 = [SELECT Id, LastName FROM Contact WHERE Id = :c3.Id];
Contact cn1 = [SELECT Id, LastName, Account.Name FROM Contact WHERE Id = :c3.Id];
rows.add('nullParent=' + (cn1.Account == null) + '|sameHash=' + (cn0.hashCode() == cn1.hashCode()));
for (String r : rows) { System.debug('OBS|' + r); }
