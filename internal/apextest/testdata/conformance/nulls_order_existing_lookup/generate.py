import collections,csv,json,pathlib
base=pathlib.Path('.');refs=json.loads((base/'refs.json').read_text());assert len(refs)==269
(base/'refs.json').write_text(json.dumps(refs));by=collections.OrderedDict()
for r in refs:by.setdefault(r['obj'],[]).append(r['field'])
classes=base/'proj/force-app/main/default/classes';classes.mkdir(parents=True,exist_ok=True);(base/'proj/sfdx-project.json').write_text('{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}')
core=r'''@IsTest(SeeAllData=true) public class NullExistingCore%(v)s {
 public static String probeObject(String obj, List<String> fields) {
  Map<String,Object> out=new Map<String,Object>();
  for(String f:fields) {
   Map<String,Object> row=new Map<String,Object>();
   try {
    List<SObject> ns=Database.query('SELECT Id FROM '+obj+' WHERE '+f+' = null LIMIT 1',AccessLevel.SYSTEM_MODE);
    List<SObject> vs=Database.query('SELECT Id FROM '+obj+' WHERE '+f+' != null LIMIT 1',AccessLevel.SYSTEM_MODE);
    if(ns.isEmpty() || vs.isEmpty()) {row.put('skip','no existing null/non-null pair');out.put(f,row);continue;}
    Set<Id> ids=new Set<Id>{ns[0].Id,vs[0].Id};Map<Id,String> labels=new Map<Id,String>{ns[0].Id=>'N',vs[0].Id=>'V'};
    for(String tail:new List<String>{'NULLS FIRST','NULLS LAST','ASC','DESC','DESC NULLS LAST'}) {
     List<String> result=new List<String>();for(SObject x:Database.query('SELECT Id FROM '+obj+' WHERE Id IN :ids ORDER BY '+f+' '+tail,AccessLevel.SYSTEM_MODE)) result.add(labels.get(x.Id));row.put(tail,String.join(result,''));
    }
   } catch(Exception e) {row.put('skip','EXC '+e.getTypeName()+': '+e.getMessage().left(160));}
   out.put(f,row);
  }
  return JSON.serialize(out);
 }
}'''
meta='<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s.0</apiVersion><status>Active</status></ApexClass>\n'
units=[(o,fs[i:i+5]) for o,fs in by.items() for i in range(0,len(fs),5)]
for v in ['62','67']:
 (classes/('NullExistingCore'+v+'.cls')).write_text(core%{'v':v});(classes/('NullExistingCore'+v+'.cls-meta.xml')).write_text(meta%v)
 for ci in range(0,len(units),20):
  name='NullProbe'+v+'_'+format(ci//20,'02d');lines=['@IsTest(SeeAllData=true) private class '+name+' {']
  for j,(obj,fields) in enumerate(units[ci:ci+20]):
   quoted=','.join("'"+f+"'" for f in fields);lines.append(" @IsTest static void m%02d(){ System.assert(false,'CAPTURE|%s|'+NullExistingCore%s.probeObject('%s',new List<String>{%s})); }"%(j,obj,v,obj,quoted))
  lines.append('}');(classes/(name+'.cls')).write_text('\n'.join(lines)+'\n');(classes/(name+'.cls-meta.xml')).write_text(meta%v)
print('targets',len(refs),'objects',len(by),'units',len(units))
