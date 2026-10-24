List<String> names = new List<String>{'Account', 'account', 'ACCOUNT'};
for (String name : names) {
    String resolved = '';
    for (Integer i = 0; i < 12; i++) {
        resolved = Schema.getGlobalDescribe().get(name).getDescribe().getName();
    }
    System.debug('ROW\t' + name + '\t' + resolved);
}
System.debug('ROW\tmissing\t' + Schema.getGlobalDescribe().containsKey('AbsentWidget__c'));
