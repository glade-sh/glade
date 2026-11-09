@IsTest private class WhatIdTie62 {
 @IsTest static void probe(){
  List<String> eight=new List<String>{'DevopsActivityLog','DevopsEnvDeployment','DevopsEnvDeploymentMbr','DevopsEnvironment','DevopsRequestInfo','PayGroup','PayPeriod','SfdcPartnerSbscrOffer'};
  Map<String,Object> o=new Map<String,Object>();
  List<String> t=new List<String>(); for(Schema.SObjectType s:Task.WhatId.getDescribe().getReferenceTo()) t.add(String.valueOf(s)); t.sort(); o.put('task',t);
  List<String> e=new List<String>(); for(Schema.SObjectType s:Event.WhatId.getDescribe().getReferenceTo()) e.add(String.valueOf(s)); e.sort(); o.put('event',e);
  Set<String> g=Schema.getGlobalDescribe().keySet(); Map<String,Boolean> f=new Map<String,Boolean>(); for(String n:eight) f.put(n,g.contains(n.toLowerCase())); o.put('global8',f); o.put('globalCount',g.size());
  Organization org=[SELECT InstanceName, OrganizationType FROM Organization]; o.put('instance',org.InstanceName); o.put('orgType',org.OrganizationType);
  System.assert(false,'CAPTURE|'+JSON.serialize(o));
 }
}
