Set<String> k = Schema.SObjectType.KsA__c.fields.getMap().keySet();
String r = '';
try { k.remove('id'); r += 'remove=ok;'; } catch (Exception e) { r += 'remove=' + e.getTypeName() + ';'; }
try { k.addAll(new Set<String>{'zz'}); r += 'addAll=ok;'; } catch (Exception e) { r += 'addAll=' + e.getTypeName() + ';'; }
try { k.clear(); r += 'clear=ok;'; } catch (Exception e) { r += 'clear=' + e.getTypeName() + ';'; }
try { k.retainAll(new Set<String>{'id'}); r += 'retainAll=ok;'; } catch (Exception e) { r += 'retainAll=' + e.getTypeName() + ';'; }
try { k.removeAll(new Set<String>{'id'}); r += 'removeAll=ok;'; } catch (Exception e) { r += 'removeAll=' + e.getTypeName() + ';'; }
Set<String> c = new Set<String>(k); c.add('aaa'); c.add('zzz');
System.debug('OBS|ks_set_ops|' + r + c.size() + '|' + c);