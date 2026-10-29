from pathlib import Path
import xml.etree.ElementTree as E
import copy
root=Path('internal/apextest/testdata/conformance/flow_null_comparisons')
r=root/'routes'; r.mkdir(exist_ok=True)
(r/'routes.tsv').write_text('route\tshape\n'+'\n'.join([
 'typed\ttyped Flow.Interview constructor', 'action\tInvocable.Action flow invocation', 'subflow\tautolaunched parent calls numeric Decision child',
 'before_update\tbefore-save update', 'before_upsert_insert\tbefore-save upsert insert', 'before_upsert_update\tbefore-save upsert update',
 'after_insert\tafter-save insert', 'after_update\tafter-save update', 'after_upsert_insert\tafter-save upsert insert', 'after_upsert_update\tafter-save upsert update',
 'before_delete\tbefore-delete Contact observation', 'async\tcommitted deferred AsyncAfterCommit execution; suppressed in IsTest'])+'\n')
p=r/'project'; d=p/'force-app/main/default'; (d/'flows').mkdir(parents=True,exist_ok=True); (d/'classes').mkdir(exist_ok=True)
(p/'sfdx-project.json').write_text('{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}\n')
ns='http://soap.sforce.com/2006/04/metadata'; E.register_namespace('',ns)
def parse(s): return E.fromstring('<root xmlns="'+ns+'">'+s+'</root>')
def tag(n): return '{'+ns+'}'+n
def name(el): return el.findtext(tag('name'))
def node(n,body): return '<'+n+'>'+body+'</'+n+'>'
def geom(n): return '<name>'+n+'</name><label>'+n+'</label><locationX>0</locationX><locationY>0</locationY>'
def connect(n): return '<connector><targetReference>'+n+'</targetReference></connector>'
def emit(flow, fname):
 E.indent(flow)
 (d/'flows'/f'{fname}.flow-meta.xml').write_text('<?xml version="1.0" encoding="UTF-8"?>\n'+E.tostring(flow,encoding='unicode')+'\n')
base=E.parse(root/'metadata/api62/flows/NullCompareTrigger.flow-meta.xml').getroot()
for fname,kind,mode in [('NullRouteBefore','RecordBeforeSave','CreateAndUpdate'),('NullRouteAfter','RecordAfterSave','CreateAndUpdate'),('NullRouteDelete','RecordBeforeDelete','Delete'),('NullRouteAsync','RecordAfterSave','Create')]:
 flow=copy.deepcopy(base)
 for el in flow.findall(tag('interviewLabel'))+flow.findall(tag('label')):el.text=fname
 start=flow.find(tag('start'));start.find(tag('filters')+'/'+tag('value')+'/'+tag('stringValue')).text=fname
 start.find(tag('recordTriggerType')).text=mode;start.find(tag('triggerType')).text=kind
 if fname=='NullRouteAsync':
  start.remove(start.find(tag('connector')))
  path=parse('<scheduledPaths><name>Later</name>'+connect('InitNull')+'<pathType>AsyncAfterCommit</pathType></scheduledPaths>')[0]
  start.insert(len(start)-1,path)
 if kind!='RecordBeforeSave':
  for el in list(flow):
   if el.tag==tag('assignments') and name(el) in ('match','other'):
    outcome=name(el); idx=list(flow).index(el); flow.remove(el)
    creates='<recordCreates>'+geom(outcome)
    for field,value,typ in [('LastName',fname,'stringValue'),('Department','$Record.AccountNumber','elementReference'),('Description',outcome,'stringValue')]:
     creates+='<inputAssignments><field>'+field+'</field><value>'+node(typ,value)+'</value></inputAssignments>'
    creates+='<object>Contact</object><storeOutputAutomatically>true</storeOutputAutomatically></recordCreates>'
    flow.insert(idx,parse(creates)[0])
 emit(flow,fname)
auto=E.parse(root/'metadata/api62/flows/NullCompareAuto.flow-meta.xml').getroot()
# A parent with only bindings: the captured child owns every numeric Decision.
flow=parse('<Flow><apiVersion>62.0</apiVersion><interviewLabel>NullRouteParent</interviewLabel><label>NullRouteParent</label><processType>AutoLaunchedFlow</processType><start><locationX>0</locationX><locationY>0</locationY>'+connect('CallChild')+'</start><status>Active</status><subflows>'+geom('CallChild')+'<flowName>NullCompareAuto</flowName><inputAssignments><name>CaseId</name><value><elementReference>CaseId</elementReference></value></inputAssignments><inputAssignments><name>InputRecord</name><value><elementReference>InputRecord</elementReference></value></inputAssignments><outputAssignments><assignToReference>Outcome</assignToReference><name>Outcome</name></outputAssignments></subflows></Flow>')[0]
for el in auto.findall(tag('variables')):
 if name(el) in ('CaseId','InputRecord','Outcome'):flow.append(copy.deepcopy(el))
emit(flow,'NullRouteParent')
