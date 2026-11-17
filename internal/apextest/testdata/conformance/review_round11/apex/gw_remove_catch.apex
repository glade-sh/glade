Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); String o = 'none';
try { gd.remove('account'); o = 'removed'; } catch (Exception e) { o = 'caught:' + e.getTypeName(); }
o += '|' + gd.containsKey('account');
System.debug('OBS|gw_remove_catch|' + o);