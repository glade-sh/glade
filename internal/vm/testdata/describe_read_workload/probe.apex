for (Schema.SObjectType token : new List<Schema.SObjectType>{Account.SObjectType, Contact.SObjectType, Opportunity.SObjectType, Task.SObjectType, User.SObjectType}) {
    Schema.DescribeSObjectResult description = token.getDescribe();
    String name = description.getName();
    Map<String,Schema.SObjectField> fields = description.fields.getMap();
    System.debug('ROW\t' + name + '\t' + fields.containsKey('Id') + ':' + fields.get('Id').getDescribe().getName() + ':' + token.getDescribe().getName());
}
Map<String,Schema.SObjectType> globals = Schema.getGlobalDescribe();
System.debug('ROW\tglobalRead\t' + (globals.get('ACCOUNT') == Account.SObjectType) + ':' + globals.keySet().contains('account'));
Map<String,Schema.SObjectType> alias = globals;
System.debug('ROW\tglobalAliasRead\t' + (alias.get('Account') == globals.get('Account')));
Map<String,Schema.SObjectType> copied = globals.clone();
System.debug('ROW\tglobalCloneRead\t' + (copied.get('Account') == Account.SObjectType));
System.debug('ROW\tglobalCloneSize\t' + (copied.size() == globals.size()));
