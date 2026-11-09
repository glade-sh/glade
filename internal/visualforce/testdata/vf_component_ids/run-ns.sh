#!/bin/bash
# Usage: run.sh <api> <org alias>   (namespaced dev org: normal HOME, generic keychain unset; NS names the org namespace)
# Deploys components, then pages (removing and recording every named failure, re-deploying),
# then renders each deployed page with PageReference.getContent through anonymous Apex.
set -u
API=$1; ORG=$2; HERE=$(cd "$(dirname "$0")" && pwd)
W=$(mktemp -d); python3 "$HERE/gen-ns.py" "$API" "$W/project" "${NS:-gladens001}"
cd "$W/project"
D=force-app/main/default
: > "$W/errors.tsv"
deploy() { # dir tag
  for round in 1 2 3 4 5 6; do
    [ -d "$1" ] && [ -n "$(ls "$1" 2>/dev/null)" ] || return 0
    sf project deploy start --ignore-conflicts --source-dir "$1" --api-version "$API" --target-org "$ORG" --wait 20 --json > "$W/$2$round.json" 2>&1
    python3 - "$W/$2$round.json" "$W/errors.tsv" "$1" <<'PY'
import json,os,sys
d=json.load(open(sys.argv[1]));r=d.get("result",{})
print("DEPLOY",sys.argv[3],r.get("status"),r.get("numberComponentsDeployed"),r.get("numberComponentErrors"))
fs=r.get("details",{}).get("componentFailures") or []
if isinstance(fs,dict): fs=[fs]
if not fs:
    if r.get("status")!="Succeeded": print("UNEXPECTED",d.get("message")); sys.exit(0)
    sys.exit(1)
with open(sys.argv[2],"a") as out:
    for f in fs:
        n=f["fullName"]
        out.write(n+"\t"+(f.get("problem") or "").replace("\n"," ")+"\n")
        for sub in ("pages","components","classes"):
            for ext in (".page",".page-meta.xml",".component",".component-meta.xml",".cls",".cls-meta.xml"):
                try: os.remove(os.path.join("force-app/main/default",sub,n+ext))
                except FileNotFoundError: pass
PY
    [ $? -ne 0 ] && return 0
  done
}
deploy $D/objects objects
deploy $D/classes classes
deploy $D/staticresources res
deploy $D/components comps
deploy $D/pages pages
python3 - <<PY
import csv
failed={l.split("\t")[0] for l in open("$W/errors.tsv")}
ok=[]
for l in open("$W/project/cases.map"):
    case,page,comp=l.rstrip("\n").split("\t")
    if page not in failed: ok.append(page)
open("$W/ok.txt","w").write("\n".join(ok))
print("deployed",len(ok),"failed",len(failed))
PY
sort -o "$W/errors.tsv" "$W/errors.tsv"
python3 - <<PY
ok=open("$W/ok.txt").read().split()
lit=",".join("'%s'"%n for n in ok)
body="""for (String n : new List<String>{"""+lit+"""}) {
 String body;
 try { body = new PageReference('/apex/' + n).getContent().toString(); } catch (Exception e) { System.debug('PICKROW|' + n + '|EXC|' + EncodingUtil.base64Encode(Blob.valueOf(e.getTypeName() + ': ' + e.getMessage()))); continue; }
 System.debug('PICKROW|' + n + '|RAW|' + EncodingUtil.base64Encode(Blob.valueOf(body)));
}
"""
open("$W/observe.apex","w").write(body)
PY
sf apex run --file "$W/observe.apex" --api-version "$API" --target-org "$ORG" --json > "$W/observe.json" 2>&1
cp "$W/observe.apex" "$HERE/observe-ns.apex"
python3 - <<PY
import json,re,base64,html
d=json.load(open("$W/observe.json"))
logs=d.get("result",{}).get("logs","") or ""
rows=[]
for m in re.finditer(r"PICKROW\|([^|]+)\|([A-Z]+)\|([A-Za-z0-9+/=]+)",html.unescape(logs)):
    rows.append((m.group(1),m.group(2),base64.b64decode(m.group(3)).decode("utf-8","replace")))
open("$W/rows.json","w").write(json.dumps(rows))
print("observed rows",len(rows), "success", d.get("result",{}).get("success"), d.get("message"))
PY
echo "$W"
