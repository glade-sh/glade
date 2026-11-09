  ProbeList__c missing = ProbeList__c.getValues('absent');
  System.debug('CS_LIST_NULL='+(missing == null));
  try { String v = missing.Text__c; System.debug('CS_LIST_VALUE='+v); } catch(Exception e) { System.debug('CS_LIST_EXCEPTION='+e.getTypeName()+'|'+e.getMessage()+'|'+e.getLineNumber()); }
  ProbeList__c instance = ProbeList__c.getInstance('absent');
  try { String v = instance.Text__c; System.debug('CS_INSTANCE_VALUE='+v); } catch(Exception e) { System.debug('CS_INSTANCE_EXCEPTION='+e.getTypeName()+'|'+e.getMessage()+'|'+e.getLineNumber()); }
  ProbeHierarchy__c h = ProbeHierarchy__c.getValues(UserInfo.getUserId());
  System.debug('CS_HVALUES_NULL='+(h == null));
  try { String v = h.Text__c; System.debug('CS_HVALUES_VALUE='+v); } catch(Exception e) { System.debug('CS_HVALUES_EXCEPTION='+e.getTypeName()+'|'+e.getMessage()+'|'+e.getLineNumber()); }
  System.debug('CS_HINSTANCE_NULL='+(ProbeHierarchy__c.getInstance()==null));
  System.debug('CS_HDEFAULT_NULL='+(ProbeHierarchy__c.getOrgDefaults()==null));
  for (NamedCredential n : [SELECT DeveloperName,MasterLabel,Endpoint,PrincipalType,NamespacePrefix FROM NamedCredential WHERE DeveloperName = 'ProbeEndpoint']) System.debug('NC_ROW='+JSON.serialize(n));
  System.debug('NC_MISSING='+[SELECT count() FROM NamedCredential WHERE DeveloperName = 'ProbeMissing']);
System.debug('NAMED_START');
Probe62.run();
