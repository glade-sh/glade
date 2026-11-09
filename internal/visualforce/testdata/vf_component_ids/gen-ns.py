#!/usr/bin/env python3
"""Namespaced variant (Developer Edition org with a namespace). Usage: gen-ns.py <api> <outdir> <namespace>
Source refers to its own custom objects, fields and Apex types without the namespace prefix. Components and pages are
CedarNC<case> / CedarNP<case>; the artifact API comes from CEDAR_ARTIFACT_API as in gen.py."""
import os, sys, shutil
api, out, ns = sys.argv[1], sys.argv[2], sys.argv[3]
art = os.environ.get('CEDAR_ARTIFACT_API') or api
shutil.rmtree(out, ignore_errors=True)
os.makedirs(out)
NS = 'xmlns="http://soap.sforce.com/2006/04/metadata"'
H = '<?xml version="1.0" encoding="UTF-8"?>\n'
def w(rel, body):
    p = os.path.join(out, "force-app/main/default", rel); os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w", encoding="utf-8").write(body)
open(os.path.join(out, "sfdx-project.json"), "w").write('{\n  "packageDirectories": [{"path": "force-app", "default": true}],\n  "namespace": "%s",\n  "sourceApiVersion": "%s"\n}\n' % (ns, api))
mp = open(os.path.join(out, "cases.map"), "w")
def cls(name, body):
    w(f"classes/{name}.cls", body + "\n")
    w(f"classes/{name}.cls-meta.xml", H + f'<ApexClass {NS}><apiVersion>{api}</apiVersion><status>Active</status></ApexClass>\n')
cls("CedarIface", "public interface CedarIface { String name(); }")
cls("CedarEnum", "public enum CedarEnum { A, B }")
cls("CedarCls", "public class CedarCls { public String s { get; set; } }")
cls("CedarCtl", "public class CedarCtl { public List<Account> accs { get; set; } public CedarCtl() { accs = new List<Account>{ new Account(Name = 'x') }; } }")
w("objects/CedarObj__c/CedarObj__c.object-meta.xml", H + f'<CustomObject {NS}><label>Cedar Obj</label><pluralLabel>Cedar Objs</pluralLabel><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>\n')
w("objects/Account/fields/CedarRel__c.field-meta.xml", H + f'<CustomField {NS}><fullName>CedarRel__c</fullName><label>Cedar Rel</label><referenceTo>Contact</referenceTo><relationshipLabel>Cedar Rels</relationshipLabel><relationshipName>CedarRel</relationshipName><type>Lookup</type><deleteConstraint>SetNull</deleteConstraint></CustomField>\n')
def meta(kind, name):
    tag = "ApexPage" if kind == "page" else "ApexComponent"
    return H + f'<{tag} {NS}><apiVersion>{art}</apiVersion><label>{name}</label></{tag}>\n'
rows = []
for line in open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "cases-ns.tsv"), encoding="utf-8").read().splitlines()[1:]:
    case, where, markup = line.split("\t", 2)
    rows.append((case, where, markup.replace("%NS%", ns)))
for case, where, markup in rows:
    page, comp = "CedarNP" + case, ""
    if where == "comp":
        comp = "CedarNC" + case
        w(f"components/{comp}.component", f'<apex:component>\n{markup}\n</apex:component>\n')
        w(f"components/{comp}.component-meta.xml", meta("component", comp))
        w(f"pages/{page}.page", f'<apex:page showHeader="false" sidebar="false" standardStylesheets="false">\n<div data-case="{page}"><c:{comp}/></div>\n</apex:page>\n')
    elif where == "std":
        w(f"pages/{page}.page", f'<apex:page standardController="Account" showHeader="false" sidebar="false" standardStylesheets="false">\n<div data-case="{page}">{markup}</div>\n</apex:page>\n')
    else:
        w(f"pages/{page}.page", f'<apex:page controller="CedarCtl" showHeader="false" sidebar="false" standardStylesheets="false">\n<div data-case="{page}">{markup}</div>\n</apex:page>\n')
    w(f"pages/{page}.page-meta.xml", meta("page", page))
    mp.write(f"{case}\t{page}\t{comp}\n")
