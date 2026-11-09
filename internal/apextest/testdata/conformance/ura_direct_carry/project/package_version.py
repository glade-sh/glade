#!/usr/bin/env python3
"""Generate the LC2PV<api> Package.Version probe classes (namespaced dev org)."""
import os, sys
OUT = sys.argv[1]
DIRECT = "SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:rid"
os.makedirs(OUT, exist_ok=True)
for api in ('62.0', '66.0', '67.0'):
    v = api[:2]
    body = f'''@IsTest private class LC2PV{v} {{
 static Object d(Id uid, Id rid) {{ try {{ List<UserRecordAccess> q=[{DIRECT}]; if(q.isEmpty()) return null; List<Object> r=new List<Object>(); for(String f:new List<String>{{'HasReadAccess','HasEditAccess','HasDeleteAccess','HasTransferAccess','HasAllAccess','MaxAccessLevel'}}) r.add(q[0].get(f)); return r; }} catch(Exception e) {{ return 'error:'+e.getTypeName()+':'+e.getMessage(); }} }}
 @IsTest static void pv() {{
  Map<String,Object> p=new Map<String,Object>();
  Account a=new Account(Name='LC2 pv'); insert a;
  Order o=new Order(AccountId=a.Id,EffectiveDate=Date.today(),Status='Draft'); insert o;
  Id admin=UserInfo.getUserId(); Id rid=o.Id;
  Profile pr=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1];
  User u=new User(Username='lc2pv.'+System.now().getTime()+'@example.invalid',Alias='lc2p',Email='lc2@example.invalid',LastName='LC2PV',ProfileId=pr.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  System.runAs(new User(Id=admin)) {{
   insert u;
   PermissionSet ps=new PermissionSet(Name='LC2PV_'+Math.abs(Crypto.getRandomInteger()),Label='LC2PV'); insert ps;
   insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Account',PermissionsRead=true);
   insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Order',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true,PermissionsDelete=true);
   insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id);
  }}
  p.put('owd',[SELECT InternalSharingModel FROM EntityDefinition WHERE QualifiedApiName='Order'].InternalSharingModel);
  p.put('A_plain',d(admin,rid));
  System.runAs(new Version(1,0)) {{
   p.put('A_pv',d(admin,rid));
   System.runAs(new User(Id=admin)) {{ p.put('A_pv_user',d(admin,rid)); }}
   System.runAs(u) {{ p.put('R_pv_user',d(u.Id,rid)); }}
  }}
  System.runAs(new User(Id=admin)) {{ p.put('A_user',d(admin,rid)); }}
  System.runAs(u) {{ p.put('R_user',d(u.Id,rid)); }}
  System.assert(false,'LC2PV|'+JSON.serialize(p));
 }}
}}
'''
    open(os.path.join(OUT, f'LC2PV{v}.cls'), 'w').write(body)
    open(os.path.join(OUT, f'LC2PV{v}.cls-meta.xml'), 'w').write(f'<?xml version="1.0" encoding="UTF-8"?>\n<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>{api}</apiVersion><status>Active</status></ApexClass>\n')
