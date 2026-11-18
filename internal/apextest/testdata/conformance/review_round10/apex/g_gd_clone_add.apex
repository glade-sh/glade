Set<String> c = Schema.getGlobalDescribe().keySet().clone(); c.add('zzz_added');
System.debug('OBS|g_gd_clone_add|' + c.contains('zzz_added'));