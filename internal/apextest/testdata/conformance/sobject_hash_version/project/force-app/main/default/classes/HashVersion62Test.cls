@IsTest private class HashVersion62Test { @IsTest static void observe() {
Map<String,Object> o = new Map<String,Object>();
o.put('H01',new List<Integer>{System.hashCode(new Account()),System.hashCode(new Contact()),System.hashCode(new Opportunity()),System.hashCode(new Lead()),System.hashCode(new Case())});
o.put('H02',new List<Integer>{System.hashCode(new Account(Name='Synthetic')),System.hashCode(new Contact(LastName='Synthetic')),System.hashCode(new Opportunity(Name=null)),System.hashCode(new Lead(LastName=null))});
o.put('H03',new List<Integer>{System.hashCode(new Account(Name='Other')),System.hashCode(new Account(Name='Synthetic',Industry='Other'))});
try {System.hashCode(null);o.put('H04','accepted');} catch(Exception e){o.put('H04',e.getTypeName()+':'+e.getMessage());}
System.assert(false,'OBS|' + JSON.serialize(o));
} }
