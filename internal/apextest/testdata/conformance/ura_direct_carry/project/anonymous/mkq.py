import json,sys,html
line=open(sys.argv[1]).read().strip()
ids=json.loads(html.unescape(line).split('LC2IDS|',1)[1])
out=["User r=[SELECT Id FROM User WHERE LastName='LC2Anon' LIMIT 1]; Id admin=UserInfo.getUserId();"]
out.append("List<String> fs=new List<String>{'HasReadAccess','HasEditAccess','HasDeleteAccess','HasTransferAccess','HasAllAccess','MaxAccessLevel'};")
for k,v in ids.items():
    if not isinstance(v,str) or not (k.endswith('|other') or k.endswith('|own')) or ':' in v: continue
    for who in ('R','A'):
        uid = 'r.Id' if who=='R' else 'admin'
        out.append(f"{{ Id uid={uid}; Id rid='{v}'; List<UserRecordAccess> q=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:rid]; List<Object> vv=new List<Object>(); if(!q.isEmpty()) for(String f:fs) vv.add(q[0].get(f)); System.debug('LC2ANON|{k}|{who}|'+JSON.serialize(q.isEmpty()?null:vv)); }}")
print('\n'.join(out))
