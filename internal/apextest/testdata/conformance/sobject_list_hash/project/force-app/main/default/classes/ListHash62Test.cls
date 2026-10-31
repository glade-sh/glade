@IsTest private class ListHash62Test { @IsTest static void observe() {
Map<String,Object> o = new Map<String,Object>();
o.put('L01', new List<Integer>{System.hashCode(new List<Account>{new Account()}),System.hashCode(new List<Contact>{new Contact()}),System.hashCode(new List<Opportunity>{new Opportunity()}),System.hashCode(new List<Lead>{new Lead()}),System.hashCode(new List<Case>{new Case()})});
o.put('L02', new List<Integer>{System.hashCode(new List<Account>{new Account(Name='Synthetic')}),System.hashCode(new List<Contact>{new Contact(LastName='Synthetic')}),System.hashCode(new List<Opportunity>{new Opportunity(Name='Synthetic')}),System.hashCode(new List<Lead>{new Lead(LastName='Synthetic')})});
o.put('L03', new List<Integer>{System.hashCode(new List<Account>()),System.hashCode(new List<Account>{new Account(),new Account()}),System.hashCode(new List<SObject>{new Account()}),System.hashCode(new List<Account>{null})});
o.put('L04', new List<Integer>{System.hashCode(new List<Account>{new Account(Name=null)}),System.hashCode(new List<Account>{(Account)JSON.deserialize('{}',Account.class)}),System.hashCode(new List<Account>{new Account(Name='Synthetic',Industry='Other')})});
o.put('L07', new List<Integer>{System.hashCode(new List<Account>{new Account(Name='Synthetic'),new Account()}),System.hashCode(new List<SObject>{new Contact(LastName='Synthetic'),new Account(Name=null)}),System.hashCode(new List<List<Account>>{new List<Account>{new Account()}})});
Account inserted=new Account(Name='Synthetic'); insert inserted;
Account queried=[SELECT Id,Name FROM Account WHERE Id=:inserted.Id];
Account fresh=new Account(Id=inserted.Id,Name='Synthetic');
o.put('L05', new List<Boolean>{queried.equals(fresh),System.hashCode(queried)==System.hashCode(fresh),System.hashCode(new List<Account>{queried})==System.hashCode(new List<Account>{fresh})});
try {System.hashCode(null);o.put('L06','accepted');} catch(Exception e){o.put('L06',e.getTypeName()+':'+e.getMessage());}
System.assert(false, 'OBS|' + JSON.serialize(o));
} }
