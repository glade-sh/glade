Set<String> k = Schema.getGlobalDescribe().keySet(); k.add('zzz_added');
System.debug('OBS|g_gd_add|' + k.contains('zzz_added'));