Account row = new Account(Name='Birch unmatched',Phone='Draft');
insert row;
row.Phone='Ready';
Map<String,Object> observations = new Map<String,Object>();
for (String name : new List<String>{'BirchBeforeTrue','BirchBeforeFalse','BirchAfterTrue','BirchAfterFalse'}) {
 Flow.Interview interview = Flow.Interview.createInterview(name,new Map<String,Object>{'$Record'=>row});
 interview.start();
 Account returned = (Account)interview.getVariableValue('$Record');
 observations.put(name,new Map<String,Object>{'contacts'=>[SELECT COUNT() FROM Contact WHERE LastName=:name],'description'=>returned==null?null:returned.Description});
}
System.assert(false,'BIRCH|'+JSON.serialize(observations));
