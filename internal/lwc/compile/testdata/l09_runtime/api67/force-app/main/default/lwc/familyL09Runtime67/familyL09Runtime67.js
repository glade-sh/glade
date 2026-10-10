
import { LightningElement, wire } from 'lwc';
import externalUpdate from '@salesforce/apex/FamilyL09ExternalDml.externalUpdate';
import externalCreate from '@salesforce/apex/FamilyL09ExternalDml.createUncached';
import queryContext from '@salesforce/apex/FamilyL09ExternalDml.queryContext';
import { getLayout } from 'lightning/uiLayoutApi';
import NAME_FIELD from '@salesforce/schema/Account.Name';
import { getRecord, getRecords, getFieldValue, getFieldDisplayValue,
 createRecord, updateRecord, deleteRecord, generateRecordInputForCreate,
 generateRecordInputForUpdate, createRecordInputFilteredByEditedFields,
 notifyRecordUpdateAvailable, getRecordNotifyChange } from 'lightning/uiRecordApi';
export default class FamilyL09Runtime67 extends LightningElement {
 queryContextMetadata;
 requested=''; result=''; ownedState='[]'; owned=[]; tokens={}; serial=0;
 recordIdFields;recordIdLayouts;recordIdUnion;activeRead='fields';
 fields; optionalFields; layoutTypes; modes; records;
 pendingOne; pendingMany; latestOne; busy=false;
 @wire(getRecord,{recordId:'$recordIdFields',fields:'$fields',optionalFields:'$optionalFields'})
 oneFields(value){if(this.activeRead==='fields')this.one(value);}
 @wire(getRecord,{recordId:'$recordIdLayouts',layoutTypes:'$layoutTypes',modes:'$modes'})
 oneLayouts(value){if(this.activeRead==='layouts')this.one(value);}
 @wire(getRecord,{recordId:'$recordIdUnion',fields:'$fields',optionalFields:'$optionalFields',layoutTypes:'$layoutTypes',modes:'$modes'})
 oneUnion(value){if(this.activeRead==='union')this.one(value);}
 one(value) { this.latestOne=value; if(this.pendingOne && (value.data || value.error)) this.pendingOne(value); }
 @wire(getRecords,{records:'$records'})
 many(value) { if(this.pendingMany && (value.data || value.error)) this.pendingMany(value); }
 async tick(){ await new Promise(resolve=>setTimeout(resolve,50)); }
 async read(id,config){
  this.recordIdFields=undefined;this.recordIdLayouts=undefined;this.recordIdUnion=undefined;
  this.pendingOne=null;await this.tick();
  return new Promise((resolve)=>{
   const emptySelection=Array.isArray(config.fields) && config.fields.length===0 &&
    !(Array.isArray(config.layoutTypes) && config.layoutTypes.length);
   const bounded=id===null || id===undefined || id==='' || config.fields===null || emptySelection;
   const timer=setTimeout(()=>{
    this.pendingOne=null;
    let observation;
    if(emptySelection)observation={noDataOrErrorWithinMs:4000};
    else if(bounded)observation={noEmissionWithinMs:4000};
    else observation={noDataOrErrorWithinMs:30000};
    this.latestOne=observation;resolve(observation);
   },bounded?4000:30000);
   this.pendingOne=value=>{clearTimeout(timer);this.pendingOne=null;resolve(value);};
   this.fields=config.fields===undefined?[]:config.fields;
   this.optionalFields=config.optionalFields===undefined?[]:config.optionalFields;
   this.layoutTypes=config.layoutTypes===undefined?[]:config.layoutTypes;
   this.modes=config.modes===undefined?['View']:config.modes;
   this.activeRead=config.layoutTypes===undefined?'fields':config.fields===undefined?'layouts':'union';
   if(this.activeRead==='fields')this.recordIdFields=id;
   else if(this.activeRead==='layouts')this.recordIdLayouts=id;
   else this.recordIdUnion=id;
  });
 }
 async readMany(records){
  this.records=undefined;this.pendingMany=null;await this.tick();
  return new Promise(resolve=>{
   const timer=setTimeout(()=>{this.pendingMany=null;resolve({noDataOrErrorWithinMs:30000});},30000);
   this.pendingMany=value=>{clearTimeout(timer);this.pendingMany=null;resolve(value);};this.records=records;
  });
 }
 async make(fields){
  const prefix='L09_ORACLE_'+this.requested+'_'+String(Date.now())+'_'+String(++this.serial);
  const record=await createRecord({apiName:'Account',fields:{Name:prefix,...fields}});
  this.owned.push(record.id);this.tokens[record.id]='[OWNED_'+this.owned.length+']';
  this.tokens[record.id.slice(0,15)]=this.tokens[record.id];
  this.ownedState=JSON.stringify(this.owned);await this.tick();return record;
 }

 contextObject;contextLayout;contextMode;pendingLayoutContext;
 @wire(getLayout,{objectApiName:'$contextObject',layoutType:'$contextLayout',mode:'$contextMode'})
 layoutContextValue(value){if(this.pendingLayoutContext && (value.data || value.error))this.pendingLayoutContext(value);}
 async captureLayoutContext(config){
  const layouts={};
  for(const type of config.layoutTypes)for(const mode of config.modes){
   this.contextObject=undefined;this.pendingLayoutContext=null;await this.tick();
   const value=await new Promise(resolve=>{
    const timer=setTimeout(()=>{this.pendingLayoutContext=null;resolve({unobservable:'NO_LAYOUT_VALUE_WITHIN_30000MS'});},30000);
    this.pendingLayoutContext=value=>{clearTimeout(timer);this.pendingLayoutContext=null;resolve(value);};
    this.contextLayout=type;this.contextMode=mode;this.contextObject='Account';
   });
   (layouts[type] ||= {})[mode]=value;
  }
  this.queryContextMetadata={layouts};
 }
 async makeUncached(){
  const name='L09_ORACLE_'+this.requested+'_'+String(Date.now())+'_'+String(++this.serial);
  const id=await externalCreate({name});
  this.owned.push(id);this.tokens[id]='[OWNED_'+this.owned.length+']';
  this.tokens[id.slice(0,15)]=this.tokens[id];
  this.ownedState=JSON.stringify(this.owned);await this.tick();return {id};
 }
 readContext(value,requireData){
  if(requireData && (!value.data || value.error))throw new Error('L09_REVIEW_GOOD_READ_FAILED');
  return {fieldNames:Object.keys(value.data?.fields || {}).sort(),
   errorProperties:value.error?Object.getOwnPropertyNames(value.error.body || {}).sort():[],
   record:this.projectRecord(value)};
 }
 async remove(id){
  const result=await deleteRecord(id);
  const canonical=this.owned.find(v=>v===id || v.slice(0,15)===id);
  this.owned=this.owned.filter(v=>v!==canonical);this.ownedState=JSON.stringify(this.owned);
  return result;
 }
 projectRecord(value){
  if(value.captureError)throw new Error('L09_CAPTURE_WIRE_TIMEOUT');
  if(value.error)return {error:value.error};
  if(!value.data)return value;
  const data=value.data,fields={};
  for(const name of ['Id','Name','Description','Phone','NumberOfEmployees','AnnualRevenue','ParentId']){
   if(data.fields && Object.prototype.hasOwnProperty.call(data.fields,name))fields[name]=data.fields[name];
  }
  return {apiName:data.apiName,id:data.id,fields};
 }
 projectMany(value){
  if(value.captureError)throw new Error('L09_CAPTURE_WIRE_TIMEOUT');
  if(value.error || !value.data)return value;
  return {results:value.data.results.map(item=>({statusCode:item.statusCode,
   result:item.statusCode===200?this.projectRecord({data:item.result}):item.result}))};
 }
 stable(value){
  if(value===undefined)return {undefined:true};
  if(value===null || typeof value==='number' || typeof value==='boolean')return value;
  if(typeof value==='string'){
   let text=value;
   for(const id of Object.keys(this.tokens).sort((a,b)=>b.length-a.length))text=text.split(id).join(this.tokens[id]);
   return text.replace(/L09_ORACLE_[A-Za-z0-9_]+/g,'[OWNED_NAME]')
    .replace(/https?:\/\/[^\s"<>]+/g,'[URL]')
    .replace(/\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\b/g,v=>v==='2000-01-01T00:00:00.000Z'?v:'[DATETIME]');
  }
  if(Array.isArray(value))return value.map(v=>this.stable(v));
  if(value instanceof Error)return {name:value.name,message:this.stable(value.message)};
  const out={};for(const key of Object.keys(value).sort()){
   if(/^(authorization|accessToken|sessionId|password|set-cookie)$/i.test(key))continue;
   out[key]=this.stable(value[key]);
  }return out;
 }
 async handle(event){
  if(this.busy)return;this.busy=true;this.requested=event.target.dataset.id;this.result='';
  this.queryContextMetadata=undefined;this.tokens={};this.owned=[];this.ownedState='[]';let answer;let cleanupErrors=[];let captureError=null;
  try{answer={returned:await this.execute(this.requested)};}
  catch(error){
   if(error.message==='L09_CAPTURE_WIRE_TIMEOUT')captureError='WIRE_TIMEOUT';
   answer={thrown:error};
  }
  finally{
   for(const id of [...this.owned].reverse()){
    try{await this.remove(id);}catch(error){cleanupErrors.push(this.stable(error));}
   }
   this.busy=false;
  }
  this.result=JSON.stringify({id:this.requested,observation:this.stable(answer),cleanupErrors,captureError,nativeContext:this.stable(this.queryContextMetadata)});
 }
 async execute(id){switch(id){
 case "r_dto_getFieldValue_string_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "OWNED", "displayValue": null}}}, "Account.Name");}
case "r_dto_getFieldValue_string_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "OWNED", "displayValue": null}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_string_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "OWNED", "displayValue": null}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_string_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "OWNED", "displayValue": null}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_empty_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "", "displayValue": ""}}}, "Account.Name");}
case "r_dto_getFieldValue_empty_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "", "displayValue": ""}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_empty_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "", "displayValue": ""}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_empty_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "", "displayValue": ""}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_null_value_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": null, "displayValue": null}}}, "Account.Name");}
case "r_dto_getFieldValue_null_value_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": null, "displayValue": null}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_null_value_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": null, "displayValue": null}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_null_value_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": null, "displayValue": null}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_number_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": 7, "displayValue": "SEVEN"}}}, "Account.Name");}
case "r_dto_getFieldValue_number_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": 7, "displayValue": "SEVEN"}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_number_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": 7, "displayValue": "SEVEN"}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_number_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": 7, "displayValue": "SEVEN"}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_boolean_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": false, "displayValue": "FALSE"}}}, "Account.Name");}
case "r_dto_getFieldValue_boolean_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": false, "displayValue": "FALSE"}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_boolean_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": false, "displayValue": "FALSE"}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_boolean_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": false, "displayValue": "FALSE"}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_display_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "RAW", "displayValue": "DISPLAY"}}}, "Account.Name");}
case "r_dto_getFieldValue_display_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "RAW", "displayValue": "DISPLAY"}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_display_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "RAW", "displayValue": "DISPLAY"}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_display_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "RAW", "displayValue": "DISPLAY"}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_absent_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {}}, "Account.Name");}
case "r_dto_getFieldValue_absent_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_absent_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_absent_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_null_field_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": null}}, "Account.Name");}
case "r_dto_getFieldValue_null_field_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": null}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_null_field_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": null}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_null_field_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": null}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_absent_value_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"displayValue": "DISPLAY"}}}, "Account.Name");}
case "r_dto_getFieldValue_absent_value_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"displayValue": "DISPLAY"}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_absent_value_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"displayValue": "DISPLAY"}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_absent_value_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"displayValue": "DISPLAY"}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_nested_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Parent": {"value": {"apiName": "Account", "fields": {"Name": {"value": "PARENT", "displayValue": "PARENT_DISPLAY"}}}}}}, "Account.Name");}
case "r_dto_getFieldValue_nested_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Parent": {"value": {"apiName": "Account", "fields": {"Name": {"value": "PARENT", "displayValue": "PARENT_DISPLAY"}}}}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_nested_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Parent": {"value": {"apiName": "Account", "fields": {"Name": {"value": "PARENT", "displayValue": "PARENT_DISPLAY"}}}}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_nested_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Parent": {"value": {"apiName": "Account", "fields": {"Name": {"value": "PARENT", "displayValue": "PARENT_DISPLAY"}}}}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_array_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": ["A", "B"], "displayValue": null}}}, "Account.Name");}
case "r_dto_getFieldValue_array_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": ["A", "B"], "displayValue": null}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_array_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": ["A", "B"], "displayValue": null}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_array_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": ["A", "B"], "displayValue": null}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_object_name":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": {"owned": 1}, "displayValue": null}}}, "Account.Name");}
case "r_dto_getFieldValue_object_parent":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": {"owned": 1}, "displayValue": null}}}, "Account.Parent.Name");}
case "r_dto_getFieldDisplayValue_object_name":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": {"owned": 1}, "displayValue": null}}}, "Account.Name");}
case "r_dto_getFieldDisplayValue_object_parent":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": {"owned": 1}, "displayValue": null}}}, "Account.Parent.Name");}
case "r_dto_getFieldValue_record_null":{return getFieldValue(null,"Account.Name");}
case "r_dto_getFieldValue_record_undefined":{return getFieldValue(undefined,"Account.Name");}
case "r_dto_getFieldValue_fields_null":{return getFieldValue({fields:null},"Account.Name");}
case "r_dto_getFieldValue_field_null":{return getFieldValue({fields:{}},null);}
case "r_dto_getFieldValue_field_undefined":{return getFieldValue({fields:{}},undefined);}
case "r_dto_getFieldValue_unqualified":{return getFieldValue({fields:{Name:{value:"A"}}},"Name");}
case "r_dto_getFieldDisplayValue_record_null":{return getFieldDisplayValue(null,"Account.Name");}
case "r_dto_getFieldDisplayValue_record_undefined":{return getFieldDisplayValue(undefined,"Account.Name");}
case "r_dto_getFieldDisplayValue_fields_null":{return getFieldDisplayValue({fields:null},"Account.Name");}
case "r_dto_getFieldDisplayValue_field_null":{return getFieldDisplayValue({fields:{}},null);}
case "r_dto_getFieldDisplayValue_field_undefined":{return getFieldDisplayValue({fields:{}},undefined);}
case "r_dto_getFieldDisplayValue_unqualified":{return getFieldDisplayValue({fields:{Name:{value:"A"}}},"Name");}
case "r_dto_getFieldValue_schema_token":{return getFieldValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "RAW", "displayValue": "DISPLAY"}}},NAME_FIELD);}
case "r_dto_getFieldDisplayValue_schema_token":{return getFieldDisplayValue({"apiName": "Account", "id": "001000000000001AAA", "fields": {"Name": {"value": "RAW", "displayValue": "DISPLAY"}}},NAME_FIELD);}
case "r_getRecord_name":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name"]}));}
case "r_getRecord_multiple":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name", "Account.Description", "Account.NumberOfEmployees"]}));}
case "r_getRecord_null_value":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Phone"]}));}
case "r_getRecord_numeric":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.AnnualRevenue", "Account.NumberOfEmployees"]}));}
case "r_getRecord_duplicate":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name", "Account.Name"]}));}
case "r_getRecord_optional":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name"], "optionalFields": ["Account.Phone"]}));}
case "r_getRecord_optional_missing":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name"], "optionalFields": ["Account.L09Missing__c"]}));}
case "r_getRecord_required_missing":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.L09Missing__c"]}));}
case "r_getRecord_unqualified":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Name"]}));}
case "r_getRecord_wrong_object":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Contact.LastName"]}));}
case "r_getRecord_empty_fields":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": []}));}
case "r_getRecord_null_fields":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": null}));}
case "r_getRecord_compact":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"layoutTypes": ["Compact"], "modes": ["View"]}));}
case "r_getRecord_full":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"layoutTypes": ["Full"], "modes": ["View"]}));}
case "r_getRecord_invalid_layout":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"layoutTypes": ["L09Missing"]}));}
case "r_getRecord_fields_and_layout":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name"], "layoutTypes": ["Compact"]}));}
case "r_getRecord_id15":{const a=await this.make({});return this.projectRecord(await this.read(a.id.slice(0,15),{fields:["Account.Name"]}));}
case "r_getRecord_empty":{const a=await this.make({});return this.projectRecord(await this.read("",{fields:["Account.Name"]}));}
case "r_getRecord_null":{const a=await this.make({});return this.projectRecord(await this.read(null,{fields:["Account.Name"]}));}
case "r_getRecord_undefined":{const a=await this.make({});return this.projectRecord(await this.read(undefined,{fields:["Account.Name"]}));}
case "r_getRecord_malformed":{const a=await this.make({});return this.projectRecord(await this.read("L09_BAD_ID",{fields:["Account.Name"]}));}
case "r_getRecord_deleted":{const a=await this.make({});await this.remove(a.id);return this.projectRecord(await this.read(a.id,{fields:["Account.Name"]}));}
case "r_getRecords_one":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id],fields:["Account.Name"]}]));}
case "r_getRecords_two":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id,b.id],fields:["Account.Name"]}]));}
case "r_getRecords_duplicate":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id,a.id],fields:["Account.Name"]}]));}
case "r_getRecords_two_configs":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id],fields:["Account.Name"]},{recordIds:[b.id],fields:["Account.Description"]}]));}
case "r_getRecords_empty_ids":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[],fields:["Account.Name"]}]));}
case "r_getRecords_empty_configs":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([]));}
case "r_getRecords_missing_field":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id],fields:["Account.L09Missing__c"]}]));}
case "r_getRecords_mixed_bad_id":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id,"L09_BAD_ID"],fields:["Account.Name"]}]));}
case "r_createRecord_name":{const a=await this.make({});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_description":{const a=await this.make({Description:"OWNED_DESCRIPTION"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_empty_text":{const a=await this.make({Description:""});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_null_text":{const a=await this.make({Description:null});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_zero_integer":{const a=await this.make({NumberOfEmployees:0});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_integer":{const a=await this.make({NumberOfEmployees:7});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_numeric_string":{const a=await this.make({NumberOfEmployees:"7"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_bad_integer":{const a=await this.make({NumberOfEmployees:"notNumber"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_decimal":{const a=await this.make({AnnualRevenue:12.5});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_decimal_string":{const a=await this.make({AnnualRevenue:"12.50"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_unknown_field":{const a=await this.make({L09Missing__c:"A"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_name_null":{const a=await this.make({Name:null});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_name_empty":{const a=await this.make({Name:""});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_createRecord_name_long":{const a=await this.make({Name:"X".repeat(256)});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "r_updateRecord_name":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Name:"UPDATED"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_description":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Description:"UPDATED_DESCRIPTION"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_same":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Description:"ORIGINAL"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_empty":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_clear_null":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Description:null}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_clear_empty":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Description:""}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_zero":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{NumberOfEmployees:0}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_number_string":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{NumberOfEmployees:"9"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_bad_number":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{NumberOfEmployees:"bad"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_unknown":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{L09Missing__c:"A"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_name_null":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Name:null}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_if_unmodified_stale":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Description:"NEW"}}},{ifUnmodifiedSince:"2000-01-01T00:00:00.000Z"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "r_updateRecord_input_null":{return updateRecord(null);}
case "r_updateRecord_fields_null":{return updateRecord({fields:null});}
case "r_updateRecord_id_missing":{return updateRecord({fields:{Description:"A"}});}
case "r_updateRecord_id_bad":{return updateRecord({fields:{Id:"L09_BAD_ID"}});}
case "r_deleteRecord_owned":{const a=await this.make({});return this.remove(a.id);}
case "r_deleteRecord_twice":{const a=await this.make({});await this.remove(a.id);return this.remove(a.id);}
case "r_deleteRecord_id15":{const a=await this.make({});return this.remove(a.id.slice(0,15));}
case "r_deleteRecord_null":{const a=await this.make({});return this.remove(null);}
case "r_deleteRecord_empty":{const a=await this.make({});return this.remove("");}
case "r_deleteRecord_bad":{const a=await this.make({});return this.remove("L09_BAD_ID");}
case "r_generateRecordInputForCreate_normal":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}});}
case "r_generateRecordInputForCreate_empty_fields":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{}});}
case "r_generateRecordInputForCreate_null_value":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Description:{value:null}}});}
case "r_generateRecordInputForCreate_empty_value":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Description:{value:""}}});}
case "r_generateRecordInputForCreate_zero":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{NumberOfEmployees:{value:0}}});}
case "r_generateRecordInputForCreate_number":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{AnnualRevenue:{value:12.5}}});}
case "r_generateRecordInputForCreate_display_value":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"RAW",displayValue:"DISPLAY"}}});}
case "r_generateRecordInputForCreate_readonly":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"A"},CreatedDate:{value:"2000-01-01T00:00:00.000Z"}}});}
case "r_generateRecordInputForCreate_relationship":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Parent:{value:{fields:{Name:{value:"PARENT"}}}},ParentId:{value:null}}});}
case "r_generateRecordInputForCreate_null_record":{return generateRecordInputForCreate(null);}
case "r_generateRecordInputForCreate_undefined_record":{return generateRecordInputForCreate(undefined);}
case "r_generateRecordInputForCreate_no_api":{return generateRecordInputForCreate({fields:{Name:{value:"A"}}});}
case "r_generateRecordInputForCreate_null_fields":{return generateRecordInputForCreate({apiName:"Account",fields:null});}
case "r_generateRecordInputForCreate_direct_values":{return generateRecordInputForCreate({apiName:"Account",fields:{Name:"DIRECT"}});}
case "r_generateRecordInputForCreate_null_field":{return generateRecordInputForCreate({apiName:"Account",fields:{Name:null}});}
case "r_generateRecordInputForCreate_info_writable":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}},{fields:{Name:{createable:true,updateable:true},Description:{createable:true,updateable:true}}});}
case "r_generateRecordInputForCreate_info_readonly":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}},{fields:{Name:{createable:false,updateable:false},Description:{createable:true,updateable:true}}});}
case "r_generateRecordInputForCreate_info_empty":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}},{fields:{}});}
case "r_generateRecordInputForCreate_info_null":{return generateRecordInputForCreate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}},null);}
case "r_generateRecordInputForUpdate_normal":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}});}
case "r_generateRecordInputForUpdate_empty_fields":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{}});}
case "r_generateRecordInputForUpdate_null_value":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Description:{value:null}}});}
case "r_generateRecordInputForUpdate_empty_value":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Description:{value:""}}});}
case "r_generateRecordInputForUpdate_zero":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{NumberOfEmployees:{value:0}}});}
case "r_generateRecordInputForUpdate_number":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{AnnualRevenue:{value:12.5}}});}
case "r_generateRecordInputForUpdate_display_value":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"RAW",displayValue:"DISPLAY"}}});}
case "r_generateRecordInputForUpdate_readonly":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"A"},CreatedDate:{value:"2000-01-01T00:00:00.000Z"}}});}
case "r_generateRecordInputForUpdate_relationship":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Parent:{value:{fields:{Name:{value:"PARENT"}}}},ParentId:{value:null}}});}
case "r_generateRecordInputForUpdate_null_record":{return generateRecordInputForUpdate(null);}
case "r_generateRecordInputForUpdate_undefined_record":{return generateRecordInputForUpdate(undefined);}
case "r_generateRecordInputForUpdate_no_api":{return generateRecordInputForUpdate({fields:{Name:{value:"A"}}});}
case "r_generateRecordInputForUpdate_null_fields":{return generateRecordInputForUpdate({apiName:"Account",fields:null});}
case "r_generateRecordInputForUpdate_direct_values":{return generateRecordInputForUpdate({apiName:"Account",fields:{Name:"DIRECT"}});}
case "r_generateRecordInputForUpdate_null_field":{return generateRecordInputForUpdate({apiName:"Account",fields:{Name:null}});}
case "r_generateRecordInputForUpdate_info_writable":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}},{fields:{Name:{createable:true,updateable:true},Description:{createable:true,updateable:true}}});}
case "r_generateRecordInputForUpdate_info_readonly":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}},{fields:{Name:{createable:false,updateable:false},Description:{createable:true,updateable:true}}});}
case "r_generateRecordInputForUpdate_info_empty":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}},{fields:{}});}
case "r_generateRecordInputForUpdate_info_null":{return generateRecordInputForUpdate({apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"OWNED"},Description:{value:"A"}}},null);}
case "r_filter_unchanged":{return createRecordInputFilteredByEditedFields({fields:{Id:"001000000000001AAA",Name:"ORIGINAL"}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_changed":{return createRecordInputFilteredByEditedFields({fields:{Id:"001000000000001AAA",Name:"CHANGED"}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_empty":{return createRecordInputFilteredByEditedFields({fields:{}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_same_null":{return createRecordInputFilteredByEditedFields({fields:{Description:null}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_null_to_empty":{return createRecordInputFilteredByEditedFields({fields:{Description:""}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_new_field":{return createRecordInputFilteredByEditedFields({fields:{Phone:"555"}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_numeric_same":{return createRecordInputFilteredByEditedFields({fields:{NumberOfEmployees:7}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_numeric_string":{return createRecordInputFilteredByEditedFields({fields:{NumberOfEmployees:"7"}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_numeric_changed":{return createRecordInputFilteredByEditedFields({fields:{NumberOfEmployees:8}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_missing_id":{return createRecordInputFilteredByEditedFields({fields:{Name:"CHANGED"}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_undefined_value":{return createRecordInputFilteredByEditedFields({fields:{Name:undefined}},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_filter_fields_null":{return createRecordInputFilteredByEditedFields({fields:null},{apiName:"Account",id:"001000000000001AAA",fields:{Name:{value:"ORIGINAL"},Description:{value:null},NumberOfEmployees:{value:7}}});}
case "r_notify_one":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable([{recordId:a.id}]);}
case "r_notify_duplicate":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable([{recordId:a.id},{recordId:a.id}]);}
case "r_notify_empty":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable([]);}
case "r_notify_null":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable(null);}
case "r_notify_undefined":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable(undefined);}
case "r_notify_null_id":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable([{recordId:null}]);}
case "r_notify_bad_id":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable([{recordId:"L09_BAD_ID"}]);}
case "r_notify_wrong_shape":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable([{Id:a.id}]);}
case "r_notify_string_id":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable([a.id]);}
case "r_notify_two":{const a=await this.make({});const b=await this.make({});return notifyRecordUpdateAvailable([{recordId:a.id},{recordId:b.id}]);}
case "r_cache_notify_updated":{const a=await this.make({Description:"BEFORE"});const before=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));await updateRecord({fields:{Id:a.id,Description:"AFTER"}});const result=await notifyRecordUpdateAvailable([{recordId:a.id}]);const after=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));return {before,after,result};}
case "r_cache_legacy_updated":{const a=await this.make({Description:"BEFORE"});const before=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));await updateRecord({fields:{Id:a.id,Description:"AFTER"}});const result=await getRecordNotifyChange([{recordId:a.id}]);const after=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));return {before,after,result};}
case "r_cache_notify_twice":{const a=await this.make({Description:"BEFORE"});const before=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));await updateRecord({fields:{Id:a.id,Description:"AFTER"}});const result=await notifyRecordUpdateAvailable([{recordId:a.id}]);await notifyRecordUpdateAvailable([{recordId:a.id}]);const after=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));return {before,after,result};}
case "r_cache_notify_deleted":{const a=await this.make({Description:"BEFORE"});const before=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));await this.remove(a.id);const result=await notifyRecordUpdateAvailable([{recordId:a.id}]);const after=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));return {before,after,result};}
case "r_cache_external_update":{const a=await this.make({Description:"BEFORE"});const before=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));await externalUpdate({recordId:a.id});const beforeNotify=this.projectRecord(this.latestOne);await notifyRecordUpdateAvailable([{recordId:a.id}]);const afterNotify=this.projectRecord(this.latestOne);return {before,beforeNotify,afterNotify};}
case "r_cache_external_duplicate":{const a=await this.make({Description:"BEFORE"});const before=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));await externalUpdate({recordId:a.id});const beforeNotify=this.projectRecord(this.latestOne);await notifyRecordUpdateAvailable([{recordId:a.id},{recordId:a.id}]);const afterNotify=this.projectRecord(this.latestOne);return {before,beforeNotify,afterNotify};}
case "ctrl_singleQuery_original":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.L09Missing__c"]}));}
case "ctrl_singleQuery_good":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name"]}));}
case "ctrl_batchQuery_original":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id],fields:["Account.L09Missing__c"]}]));}
case "ctrl_batchQuery_good":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id],fields:["Account.Name"]}]));}
case "ctrl_wrongObject_original":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Contact.LastName"]}));}
case "ctrl_wrongObject_good":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name"]}));}
case "ctrl_deletedRead_original":{const a=await this.make({});await this.remove(a.id);return this.projectRecord(await this.read(a.id,{fields:["Account.Name"]}));}
case "ctrl_deletedRead_good":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name"]}));}
case "ctrl_deletedNotify_original":{const a=await this.make({Description:"BEFORE"});const before=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));await this.remove(a.id);const result=await notifyRecordUpdateAvailable([{recordId:a.id}]);const after=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));return {before,after,result};}
case "ctrl_deletedNotify_good":{const a=await this.make({Description:"BEFORE"});const before=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));await updateRecord({fields:{Id:a.id,Description:"AFTER"}});const result=await notifyRecordUpdateAvailable([{recordId:a.id}]);const after=this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));return {before,after,result};}
case "ctrl_createNumber_original":{const a=await this.make({NumberOfEmployees:"notNumber"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_createNumber_good":{const a=await this.make({NumberOfEmployees:7});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_updateNumber_original":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{NumberOfEmployees:"bad"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_updateNumber_good":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{NumberOfEmployees:0}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_createField_original":{const a=await this.make({L09Missing__c:"A"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_createField_good":{const a=await this.make({Description:"OWNED_DESCRIPTION"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_updateField_original":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{L09Missing__c:"A"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_updateField_good":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Description:"UPDATED_DESCRIPTION"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_createRequired_original":{const a=await this.make({Name:null});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_createRequired_good":{const a=await this.make({});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_updateRequired_original":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Name:null}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_updateRequired_good":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Name:"UPDATED"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_deletedWrite_original":{const a=await this.make({});await this.remove(a.id);return this.remove(a.id);}
case "ctrl_deletedWrite_good":{const a=await this.make({});return this.remove(a.id);}
case "ctrl_conditionalUpdate_original":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Description:"NEW"}}},{ifUnmodifiedSince:"2000-01-01T00:00:00.000Z"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_conditionalUpdate_good":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Description:"UPDATED_DESCRIPTION"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_decimalString_original":{const a=await this.make({AnnualRevenue:"12.50"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_decimalString_good":{const a=await this.make({AnnualRevenue:12.5});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_singleQuery_otherField":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.L09ControlMissing__c"]}));}
case "ctrl_singleQuery_nameAndMissing":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Account.Name","Account.L09Missing__c"]}));}
case "ctrl_batchQuery_otherField":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id],fields:["Account.L09ControlMissing__c"]}]));}
case "ctrl_batchQuery_nameAndMissing":{const a=await this.make({});const b=await this.make({Description:"SECOND"});return this.projectMany(await this.readMany([{recordIds:[a.id],fields:["Account.Name","Account.L09Missing__c"]}]));}
case "ctrl_createField_otherField":{const a=await this.make({L09ControlMissing__c:"A"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_updateField_otherField":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{L09ControlMissing__c:"A"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_wrongObject_otherObject":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5}); return this.projectRecord(await this.read(a.id,{"fields": ["Opportunity.Name"]}));}
case "ctrl_createNumber_otherInvalid":{const a=await this.make({NumberOfEmployees:"bad"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_updateNumber_otherInvalid":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{NumberOfEmployees:"notNumber"}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_createRequired_empty":{const a=await this.make({Name:""});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_createRequired_tooLong":{const a=await this.make({Name:"X".repeat(256)});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_updateRequired_tooLong":{const a=await this.make({Description:"ORIGINAL"});await updateRecord({fields:{Id:a.id,...{Name:"X".repeat(256)}}});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees"]}));}
case "ctrl_decimalString_below":{const a=await this.make({AnnualRevenue:"12.49"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_decimalString_above":{const a=await this.make({AnnualRevenue:"12.51"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_decimalString_negative":{const a=await this.make({AnnualRevenue:"-12.51"});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_decimalString_negativeGood":{const a=await this.make({AnnualRevenue:-12.51});return this.projectRecord(await this.read(a.id,{fields:["Account.Name","Account.Description","Account.NumberOfEmployees","Account.AnnualRevenue"]}));}
case "ctrl_conditionalUpdate_currentTimestamp":{const a=await this.make({Description:"ORIGINAL"});const record=await this.read(a.id,{fields:["Account.LastModifiedDate","Account.LastModifiedBy.Name"]});if(record.error)throw record.error;await updateRecord({fields:{Id:a.id,Description:"NEW"}},{ifUnmodifiedSince:getFieldValue(record.data,"Account.LastModifiedDate")});return this.projectRecord(await this.read(a.id,{fields:["Account.Description"]}));}
case "ctrl_conditionalUpdate_modifierContext":{const a=await this.make({Description:"ORIGINAL"});const record=await this.read(a.id,{fields:["Account.LastModifiedDate","Account.LastModifiedBy.Name"]});if(record.error)throw record.error;try{await updateRecord({fields:{Id:a.id,Description:"NEW"}},{ifUnmodifiedSince:"2000-01-01T00:00:00.000Z"});return {unexpectedSuccess:true};}catch(error){return {lastModifiedBy:getFieldValue(record.data,"Account.LastModifiedBy.Name"),error};}}
case "ctrl_review_QualificationUI_crossName":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"fields":["Opportunity.Name"]});return this.readContext(value,false);}
case "ctrl_review_QualificationUI_good":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"fields":["Account.Name"]});return this.readContext(value,true);}
case "ctrl_review_QualificationUncached_crossName":{const a=await this.makeUncached();const value=await this.read(a.id,{"fields":["Opportunity.Name"]});return this.readContext(value,false);}
case "ctrl_review_QualificationUncached_good":{const a=await this.makeUncached();const value=await this.read(a.id,{"fields":["Account.Name"]});return this.readContext(value,true);}
case "ctrl_review_QualificationPrimed_crossName":{const a=await this.makeUncached();const first=await this.read(a.id,{fields:["Account.Name"]});this.readContext(first,true);const value=await this.read(a.id,{"fields":["Opportunity.Name"]});return this.readContext(value,false);}
case "ctrl_review_QualificationPrimed_good":{const a=await this.makeUncached();const first=await this.read(a.id,{fields:["Account.Name"]});this.readContext(first,true);const value=await this.read(a.id,{"fields":["Account.Name"]});return this.readContext(value,true);}
case "ctrl_review_QualificationField_crossDescription":{const a=await this.makeUncached();const first=await this.read(a.id,{fields:["Account.Name"]});this.readContext(first,true);const value=await this.read(a.id,{"fields":["Contact.Description"]});return this.readContext(value,false);}
case "ctrl_review_QualificationField_good":{const a=await this.makeUncached();const first=await this.read(a.id,{fields:["Account.Name"]});this.readContext(first,true);const value=await this.read(a.id,{"fields":["Account.Description"]});return this.readContext(value,true);}
case "ctrl_review_LayoutCompact_view":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Compact"],"modes":["View"]});const observed=this.readContext(value,true);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["View"]});return observed;}
case "ctrl_review_LayoutCompact_edit":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Compact"],"modes":["Edit"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["Edit"]});return observed;}
case "ctrl_review_LayoutCompact_create":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Compact"],"modes":["Create"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["Create"]});return observed;}
case "ctrl_review_LayoutCompact_union":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Compact"],"modes":["View","Edit"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["View","Edit"]});return observed;}
case "ctrl_review_LayoutFull_view":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Full"],"modes":["View"]});const observed=this.readContext(value,true);await this.captureLayoutContext({"layoutTypes":["Full"],"modes":["View"]});return observed;}
case "ctrl_review_LayoutFull_edit":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Full"],"modes":["Edit"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Full"],"modes":["Edit"]});return observed;}
case "ctrl_review_LayoutFull_create":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Full"],"modes":["Create"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Full"],"modes":["Create"]});return observed;}
case "ctrl_review_LayoutFull_union":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Full"],"modes":["View","Edit"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Full"],"modes":["View","Edit"]});return observed;}
case "ctrl_review_LayoutUnion_description":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"fields":["Account.Description"],"layoutTypes":["Compact"],"modes":["View"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["View"]});return observed;}
case "ctrl_review_LayoutUnion_optional":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"fields":["Account.Name"],"optionalFields":["Account.Description"],"layoutTypes":["Compact"],"modes":["View"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["View"]});return observed;}
case "ctrl_review_LayoutUnion_good":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"fields":["Account.Name"],"layoutTypes":["Compact"],"modes":["View"]});const observed=this.readContext(value,true);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["View"]});return observed;}
case "ctrl_review_LayoutEnums_badLayout":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["L09Missing"],"modes":["View"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["L09Missing"],"modes":["View"]});return observed;}
case "ctrl_review_LayoutEnums_badMode":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Compact"],"modes":["L09Missing"]});const observed=this.readContext(value,false);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["L09Missing"]});return observed;}
case "ctrl_review_LayoutEnums_good":{const a=await this.make({Description:"OWNED_DESCRIPTION",AnnualRevenue:12.5,NumberOfEmployees:7});const value=await this.read(a.id,{"layoutTypes":["Compact"],"modes":["View"]});const observed=this.readContext(value,true);await this.captureLayoutContext({"layoutTypes":["Compact"],"modes":["View"]});return observed;}
case "ctrl_review_QueryProjection_original":{const a=await this.makeUncached();this.queryContextMetadata=await queryContext();const value=await this.read(a.id,{"fields":["Account.L09Missing__c"]});return this.readContext(value,false);}
case "ctrl_review_QueryProjection_otherField":{const a=await this.makeUncached();this.queryContextMetadata=await queryContext();const value=await this.read(a.id,{"fields":["Account.L09ControlMissing__c"]});return this.readContext(value,false);}
case "ctrl_review_QueryProjection_nameAndMissing":{const a=await this.makeUncached();this.queryContextMetadata=await queryContext();const value=await this.read(a.id,{"fields":["Account.Name","Account.L09Missing__c"]});return this.readContext(value,false);}
case "ctrl_review_QueryProjection_freshField":{const a=await this.makeUncached();this.queryContextMetadata=await queryContext();const value=await this.read(a.id,{"fields":["Account.L09ReviewMissing__c"]});return this.readContext(value,false);}
case "ctrl_review_QueryProjection_primed":{const a=await this.makeUncached();this.queryContextMetadata=await queryContext();const first=await this.read(a.id,{fields:["Account.Name"]});this.readContext(first,true);const value=await this.read(a.id,{"fields":["Account.L09Missing__c"]});return this.readContext(value,false);}
case "ctrl_review_QueryProjection_good":{const a=await this.makeUncached();this.queryContextMetadata=await queryContext();const value=await this.read(a.id,{"fields":["Account.Name"]});return this.readContext(value,true);}
 case "ctrl_querySeed_original":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});this.queryContextMetadata={createdRecord:a,schema:await queryContext()};const value=await this.read(a.id,{fields:["Account.L09Missing__c"]});return this.readContext(value,false);}
case "ctrl_querySeed_otherField":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});this.queryContextMetadata={createdRecord:a,schema:await queryContext()};const value=await this.read(a.id,{fields:["Account.L09ControlMissing__c"]});return this.readContext(value,false);}
case "ctrl_querySeed_freshField":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});this.queryContextMetadata={createdRecord:a,schema:await queryContext()};const value=await this.read(a.id,{fields:["Account.L09ReviewMissing__c"]});return this.readContext(value,false);}
case "ctrl_querySeed_good":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});this.queryContextMetadata={createdRecord:a,schema:await queryContext()};const value=await this.read(a.id,{fields:["Account.Name"]});return this.readContext(value,true);}
case "ctrl_queryTrace_000":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace114__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_004":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace97__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_008":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace39__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_012":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace55__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_016":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace199__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_020":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace12__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_024":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace64__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_028":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace117__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_032":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace222__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_036":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace5__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_040":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace169__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_044":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace92__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_048":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace102__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_052":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace79__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_056":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace27__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_060":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace48__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_064":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace2__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_068":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace225__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_072":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace76__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_076":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace18__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_080":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace126__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_084":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace178__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_088":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace170__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_092":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace24__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_096":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace138__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_100":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace308__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_104":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace15__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_108":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace31__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_112":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace234__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_116":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace213__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_120":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace88__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_124":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace67__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_127":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});const value=await this.read(a.id,{fields:["Account.L09QueryTrace29__c"]});return this.readContext(value,false);}
case "ctrl_queryTrace_good":{const a=await this.make({Description:"OWNED_DESCRIPTION",NumberOfEmployees:7,AnnualRevenue:12.5});return this.readContext(await this.read(a.id,{fields:["Account.Name"]}),true);}
case "ctrl_notifySelection_external":{const a=await this.make({Description:"BEFORE"});const before=await this.read(a.id,{fields:["Account.Name"]});await externalUpdate({recordId:a.id});await notifyRecordUpdateAvailable([{recordId:a.id}]);const notified=this.projectRecord(this.latestOne);const selected=await this.read(a.id,{fields:["Account.Description"]});return {before:this.projectRecord(before),notified,selected:this.projectRecord(selected)};}
case "ctrl_notifySelection_duplicate":{const a=await this.make({Description:"BEFORE"});const before=await this.read(a.id,{fields:["Account.Name"]});await externalUpdate({recordId:a.id});await notifyRecordUpdateAvailable([{recordId:a.id},{recordId:a.id}]);const notified=this.projectRecord(this.latestOne);const selected=await this.read(a.id,{fields:["Account.Description"]});return {before:this.projectRecord(before),notified,selected:this.projectRecord(selected)};}
case "ctrl_notifySelection_good":{const a=await this.make({Description:"BEFORE"});const before=await this.read(a.id,{fields:["Account.Name"]});await notifyRecordUpdateAvailable([{recordId:a.id}]);const notified=this.projectRecord(this.latestOne);const selected=await this.read(a.id,{fields:["Account.Description"]});if(!before.data || before.error || !selected.data || selected.error)throw new Error("L09_NOTIFICATION_GOOD_READ_FAILED");return {before:this.projectRecord(before),notified,selected:this.projectRecord(selected)};}
default:throw new Error('Unknown owned row');
 }}
}
