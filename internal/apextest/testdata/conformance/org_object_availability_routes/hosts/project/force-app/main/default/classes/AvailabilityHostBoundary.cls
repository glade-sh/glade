public class AvailabilityHostBoundary { public static Map<String,Object> observe() { Map<String,Object> r=new Map<String,Object>();
try { Schema.getAppDescribe('Sales'); r.put('app',true); } catch (Exception e) { r.put('app',e.getTypeName()); }
try { Schema.getModuleDescribe(); r.put('module',true); } catch (Exception e) { r.put('module',e.getTypeName()); }
try { Schema.getModuleDescribe('Sales'); r.put('namedModule',true); } catch (Exception e) { r.put('namedModule',e.getTypeName()); }
r.put('Quote',Schema.getGlobalDescribe().containsKey('Quote'));
r.put('QuoteLineItem',Schema.getGlobalDescribe().containsKey('QuoteLineItem'));
r.put('WebStore',Schema.getGlobalDescribe().containsKey('WebStore'));
r.put('Account',Schema.getGlobalDescribe().containsKey('Account'));
return r; } }
