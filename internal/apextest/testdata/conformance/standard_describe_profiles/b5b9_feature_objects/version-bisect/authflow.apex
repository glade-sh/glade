Map<String,Schema.SObjectType> m=Schema.getGlobalDescribe();
System.debug('ROW AuthProvider.FlowDefaultAccountId '+m.get('AuthProvider').getDescribe().fields.getMap().containsKey('FlowDefaultAccountId'));
System.debug('ROW AuthProvider.FlowDefaultProfileId '+m.get('AuthProvider').getDescribe().fields.getMap().containsKey('FlowDefaultProfileId'));
