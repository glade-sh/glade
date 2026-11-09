Map<String, Schema.SObjectType> view = Schema.getGlobalDescribe();
System.debug('C358_BOUNDARY=' + (view.get('Account') != null) + ',' + (view.get('Contact') != null) + ',' + (view.get('not_a_real_object_358') == null));
