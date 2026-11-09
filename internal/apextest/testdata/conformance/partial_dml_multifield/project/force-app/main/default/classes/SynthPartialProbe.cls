public without sharing class SynthPartialProbe {
    private static List<Object> errors(Database.Error[] nativeErrors) {
        List<Object> values = new List<Object>();
        for (Database.Error item : nativeErrors) values.add(new Map<String,Object>{'getStatusCode'=>item.getStatusCode(),'getMessage'=>item.getMessage(),'getFields'=>item.getFields()});
        return values;
    }
    private static List<Object> saveResults(Database.SaveResult[] nativeResults) {
        List<Object> values = new List<Object>();
        for (Integer i=0;i<nativeResults.size();i++) {
            Database.SaveResult item=nativeResults[i];
            values.add(new Map<String,Object>{'index'=>i,'isSuccess'=>item.isSuccess(),'getId'=>item.getId(),'errors'=>errors(item.getErrors())});
        }
        return values;
    }
    private static List<Object> upsertResults(Database.UpsertResult[] nativeResults) {
        List<Object> values = new List<Object>();
        for (Integer i=0;i<nativeResults.size();i++) {
            Database.UpsertResult item=nativeResults[i];
            values.add(new Map<String,Object>{'index'=>i,'isSuccess'=>item.isSuccess(),'getId'=>item.getId(),'isCreated'=>item.isCreated(),'errors'=>errors(item.getErrors())});
        }
        return values;
    }
    private static Map<String,Object> dmlError(DmlException ex) {
        List<Object> values=new List<Object>();
        for (Integer i=0;i<ex.getNumDml();i++) values.add(new Map<String,Object>{'ordinal'=>i,'getDmlIndex'=>ex.getDmlIndex(i),'getDmlMessage'=>ex.getDmlMessage(i),'getDmlId'=>ex.getDmlId(i),'getDmlType'=>ex.getDmlType(i),'getDmlFieldNames'=>ex.getDmlFieldNames(i)});
        return new Map<String,Object>{'typeName'=>ex.getTypeName(),'message'=>ex.getMessage(),'getNumDml'=>ex.getNumDml(),'rows'=>values};
    }
    public static List<String> cases() {
        List<String> payload=new List<String>();
        String runKey=EncodingUtil.convertToHex(Crypto.generateAesKey(128));
        // Independent synthetic seeds for each case, one explicit-system setup DML.
        List<String> operations=new List<String>{'insert','update','upsertId','upsertExternal'};
        List<String> modes=new List<String>{'default','user'};
        List<String> styles=new List<String>{'partial','throw'};
        List<SynthPartial__c> seeds=new List<SynthPartial__c>();
        Map<String,List<SynthPartial__c>> caseSeeds=new Map<String,List<SynthPartial__c>>();
        for (String operation:operations) for (String mode:modes) for (String style:styles) {
            String probeId=operation+'-'+mode+'-'+style;
            List<SynthPartial__c> selected=new List<SynthPartial__c>();
            Integer count=operation=='update'?3:(operation=='insert'?0:2);
            for(Integer i=0;i<count;i++) {
                SynthPartial__c seed=new SynthPartial__c(Name='Synthetic seed '+String.valueOf(i),ExternalKey__c=runKey+'-'+probeId+'-'+String.valueOf(i));
                selected.add(seed);seeds.add(seed);
            }
            caseSeeds.put(probeId,selected);
        }
        Database.SaveResult[] seeded=Database.insert(seeds,true,AccessLevel.SYSTEM_MODE);
        for (Database.SaveResult item:seeded) if(!item.isSuccess()) throw new IllegalArgumentException('SYNTH_PDML_SETUP_FAILED');
        for (String operation:operations) for (String mode:modes) for (String style:styles) {
            String probeId=operation+'-'+mode+'-'+style;
            List<SynthPartial__c> records=new List<SynthPartial__c>();
            List<Object> inputs=new List<Object>();
            List<SynthPartial__c> selected=caseSeeds.get(probeId);
            for(Integer i=0;i<3;i++) {
                SynthPartial__c record=new SynthPartial__c(Name='Synthetic target '+String.valueOf(i));
                if(operation=='update'||(operation=='upsertId'&&i<2)) record.Id=selected[i].Id;
                if(operation=='upsertExternal') record.ExternalKey__c=runKey+'-'+probeId+'-'+String.valueOf(i);
                // The tested target-field assignment order is explicit and fixed.
                List<String> assignmentOrder=new List<String>();
                if(i==0) {
                    record.Zeta__c='synthetic-z';assignmentOrder.add('Zeta__c');
                    record.Alpha__c='synthetic-a';assignmentOrder.add('Alpha__c');
                    record.Mid__c='synthetic-m';assignmentOrder.add('Mid__c');
                } else if(i==1) { record.Zeta__c='synthetic-one';assignmentOrder.add('Zeta__c'); }
                inputs.add(new Map<String,Object>{'index'=>i,'inputRecordId'=>record.Id,'inputExternalKey'=>record.ExternalKey__c,'assignmentOrder'=>assignmentOrder,'inputRecord'=>JSON.deserializeUntyped(JSON.serialize(record))});
                records.add(record);
            }
            Map<String,Object> observation=new Map<String,Object>{'kind'=>'case','probeId'=>probeId,'operation'=>operation,'mode'=>mode,'style'=>style,'inputs'=>inputs};
            try {
                if(operation=='insert') {
                    if(style=='partial') {
                        Database.SaveResult[] results;
                        if(mode=='user') results=Database.insert(records,false,AccessLevel.USER_MODE);
                        else results=Database.insert(records,false);
                        observation.put('outcome','results');observation.put('results',saveResults(results));
                    } else {
                        if(mode=='user') { insert as user records; } else { insert records; }
                        observation.put('outcome','statementSuccess');
                    }
                } else if(operation=='update') {
                    if(style=='partial') {
                        Database.SaveResult[] results;
                        if(mode=='user') results=Database.update(records,false,AccessLevel.USER_MODE);
                        else results=Database.update(records,false);
                        observation.put('outcome','results');observation.put('results',saveResults(results));
                    } else {
                        if(mode=='user') { update as user records; } else { update records; }
                        observation.put('outcome','statementSuccess');
                    }
                } else {
                    Boolean allOrNone=style=='throw'; Database.UpsertResult[] results;
                    if(operation=='upsertExternal') {
                        if(mode=='user') results=Database.upsert(records,SynthPartial__c.Fields.ExternalKey__c,allOrNone,AccessLevel.USER_MODE);
                        else results=Database.upsert(records,SynthPartial__c.Fields.ExternalKey__c,allOrNone);
                    } else {
                        if(mode=='user') results=Database.upsert(records,allOrNone,AccessLevel.USER_MODE);
                        else results=Database.upsert(records,allOrNone);
                    }
                    observation.put('outcome','results');observation.put('results',upsertResults(results));
                }
            } catch(DmlException ex) {
                observation.put('outcome','exception');observation.put('exception',dmlError(ex));
            } catch(Exception ex) {
                // Retain unexpected operational failures, but offline gate rejects them as findings.
                observation.put('outcome','operationalException');observation.put('exception',new Map<String,Object>{'typeName'=>ex.getTypeName(),'message'=>ex.getMessage(),'stackTrace'=>ex.getStackTraceString()});
            }
            List<Id> postRecordIds=new List<Id>();for(SynthPartial__c record:records)postRecordIds.add(record.Id);
            observation.put('postRecordIds',postRecordIds);payload.add(JSON.serialize(observation));
        }
        return payload;
    }
}
