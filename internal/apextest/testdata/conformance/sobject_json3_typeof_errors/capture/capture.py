import subprocess, os, json, html, pathlib, sys
base=pathlib.Path(__file__).resolve().parent.parent
project=base/'project'
env=dict(os.environ, SF_USE_GENERIC_UNIX_KEYCHAIN='true')
org=os.environ['CAPTURE_ORG']
def sf(args, allow_failure=False):
 p=subprocess.run(['sf',*args,'--json'],cwd=project,env=env,capture_output=True,text=True)
 try: d=json.loads(p.stdout)
 except ValueError: raise RuntimeError('invalid sf JSON')
 if d.get('status') and not allow_failure: raise RuntimeError(str(d.get('message'))+' '+str(d.get('result',{}).get('details',{}).get('componentFailures',[])))
 return d.get('result',{})
shapes=['ownerAccount','ownerContact','ownerOpportunity','whoAccount','validBoundary','unknownType']
test=project/'classes/TypeofErrorCaptureTest.cls'
meta=project/'classes/TypeofErrorCaptureTest.cls-meta.xml'
probe=project/'classes/TypeofErrorProbe.cls-meta.xml'
test.write_text('@IsTest private class TypeofErrorCaptureTest {\n'+''.join("@IsTest static void row%d() { System.assert(false,'SOBJECT_TYPEOF|%s|'+JSON.serialize(TypeofErrorProbe.observe().get('%s'))); }\n"%(i,s,s) for i,s in enumerate(shapes))+'}\n')
allrows=[]
try:
 for api in ['62.0','67.0']:
  xml='<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>'+api+'</apiVersion><status>Active</status></ApexClass>\n'
  probe.write_text(xml); meta.write_text(xml)
  sf(['project','deploy','start','-o',org,'-d','classes/TypeofErrorProbe.cls','-d',str(test),'--api-version',api,'-w','10'])
  out=sf(['apex','run','-o',org,'--api-version',api,'-f',str(base/'capture/typeof.apex')])
  if not out.get('success'): raise RuntimeError('anonymous execution: '+str(out.get('exceptionMessage')))
  found={}
  for line in html.unescape(out.get('logs','')).splitlines():
   if 'USER_DEBUG' in line and 'SOBJECT_TYPEOF|' in line:
    s,v=line.split('SOBJECT_TYPEOF|',1)[1].split('|',1); found[s]=json.loads(v)['value']
  if set(found)!=set(shapes): raise RuntimeError('anonymous incomplete '+str(found))
  for s in shapes: allrows.append((api,'anonymous',s,'value',found[s]))
  out=sf(['apex','run','test','-o',org,'--api-version',api,'--class-names','TypeofErrorCaptureTest','--synchronous','--result-format','json','--wait','15'],True)
  found={}
  for t in out.get('tests',[]):
   msg=html.unescape(t.get('Message') or '')
   if 'SOBJECT_TYPEOF|' in msg:
    s,v=msg.split('SOBJECT_TYPEOF|',1)[1].split('|',1); found[s]=json.loads(v)['value']
  if set(found)!=set(shapes): raise RuntimeError('named incomplete '+str(found))
  for s in shapes: allrows.append((api,'isTest',s,'value',found[s]))
  print(api,'captured',len(found),'named and anonymous shapes',flush=True)
 (base/'org.tsv').write_text('api\troute\tshape\toperation\tvalue\n'+''.join('\t'.join(row[:4])+'\t'+json.dumps(row[4],ensure_ascii=False)+'\n' for row in allrows))
finally:
 test.unlink(missing_ok=True); meta.unlink(missing_ok=True)
 probe.write_text('<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n')
