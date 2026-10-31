@IsTest private class EmptyHashProbeTest { @IsTest static void observe() {
Map<String,Object> o = new Map<String,Object>();
Account a = new Account(Name='Synthetic'); Account b = new Account(Name='Synthetic');
o.put('H01', new List<Integer>{System.hashCode(a),System.hashCode(b)});
o.put('H02', new List<Integer>{System.hashCode(new Account()),System.hashCode(new Contact())});
o.put('H03', new List<Integer>{System.hashCode(new Account(Name='Synthetic')),System.hashCode(new Account(Name='Other')),System.hashCode(new Account(Name=null)),System.hashCode(new Account(Name='Synthetic',Industry='Other'))});
o.put('H04', new List<Integer>{System.hashCode(new Contact(LastName='Synthetic')),System.hashCode(new Contact(LastName='Synthetic')),System.hashCode(new Contact(LastName='Other'))});
o.put('T01', new Map<String,Object>{'relationship'=>Task.WhatId.getDescribe().getRelationshipName(),'targets'=>String.valueOf(Task.WhatId.getDescribe().getReferenceTo())});
System.assert(false, 'OBS|' + JSON.serialize(o));

} }
