public class CloneWriteProbe {
    public static String shape;
    public static String operation;
    public static Boolean enabled = false;
    public static Boolean caughtInside = false;
    public static Map<String,Object> observed;

    public static void afterRows(List<CloneParcel__c> rows, Map<Id,CloneParcel__c> byId) {
        if (!enabled) return;
        SObject source = rows[0];
        SObject copy;
        if (shape == 'listDefault') copy = rows.deepClone()[0];
        else if (shape == 'listPreserve') copy = rows.deepClone(true,true,true)[0];
        else if (shape == 'listClear') copy = rows.deepClone(false,false,false)[0];
        else if (shape == 'map') copy = byId.deepClone().get(rows[0].Id);
        else if (shape == 'single') copy = source.clone(true,true,true,true);
        else if (shape == 'shallowList') copy = rows.clone()[0];
        else if (shape == 'shallowMap') copy = byId.clone().get(rows[0].Id);
        else copy = source;
        CloneDepot__c depot = [SELECT Name FROM CloneDepot__c WHERE Id = :rows[0].Depot__c];
        if (operation == 'insert') {
            CloneParcel__c queried = [SELECT Depot__r.Name FROM CloneParcel__c WHERE Id = :rows[0].Id];
            depot = (CloneDepot__c)queried.getSObject('Depot__r');
        }
        try {
            if (shape == 'originalRelationship') copy.putSObject('Depot__r',depot);
            else copy.put('Name','Writable parcel');
            copy.putSObject('Depot__r',depot);
            copy.getSObject('Depot__r').put('Name','Writable depot');
            ((CloneParcel__c)copy).Name = 'Assigned parcel';
        } catch (Exception e) {
            caughtInside = true;
            throw e;
        }
        observed = new Map<String,Object>{
            'writable' => copy.get('Name') == 'Assigned parcel',
            'relationshipWritable' => copy.getSObject('Depot__r').get('Name') == 'Writable depot',
            'sourceUnchanged' => source.get('Name') == 'Parcel',
            'databaseUnchanged' => [SELECT Name FROM CloneDepot__c WHERE Id = :rows[0].Depot__c].Name == 'Depot'
        };
        if (shape == 'listDefault' || shape == 'listClear') observed.put('idCleared',copy.Id == null);
        if (shape == 'listPreserve' || shape == 'single') observed.put('idPreserved',copy.Id == source.Id);
        update new CloneDepot__c(Id=rows[0].Depot__c,Observation__c=JSON.serialize(observed));
    }

    public static Map<String,Object> run(String entry, String eventName, String variant) {
        shape = variant;
        operation = eventName;
        observed = null;
        caughtInside = false;
        CloneDepot__c depot = new CloneDepot__c(Name='Depot');
        enabled = entry == 'trigger';
        try {
            insert depot;
            if (entry != 'trigger') drive(depot.Id,eventName,variant);
        } catch (DmlException e) {
            return new Map<String,Object>{'readOnlyRejected'=>e.getMessage().contains('Record is read-only'),'caughtInside'=>caughtInside};
        }
        return observed;
    }

    public static void drive(Id depotId, String eventName, String variant) {
        shape = variant;
        operation = eventName;
        CloneParcel__c parcel = new CloneParcel__c(Name='Parcel',Depot__c=depotId);
        enabled = eventName == 'insert';
        insert parcel;
        if (eventName == 'delete') {
            enabled = true;
            delete parcel;
        }
    }

    public class Work implements Queueable {
        Id depotId;
        String eventName;
        String variant;
        public Work(Id depotId, String eventName, String variant) {
            this.depotId=depotId; this.eventName=eventName; this.variant=variant;
        }
        public void execute(QueueableContext context) { drive(depotId,eventName,variant); }
    }

    public static Map<String,Object> queueResult(String eventName, String variant) {
        CloneDepot__c depot = new CloneDepot__c(Name='Depot');
        insert depot;
        Test.startTest();
        System.enqueueJob(new Work(depot.Id,eventName,variant));
        Test.stopTest();
        return (Map<String,Object>)JSON.deserializeUntyped([SELECT Observation__c FROM CloneDepot__c WHERE Id = :depot.Id].Observation__c);
    }

    public static Map<String,Object> errors() {
        Account source = new Account(Name='Original');
        source.addError('Owned validation');
        return new Map<String,Object>{'sourceErrors'=>source.hasErrors(),
            'listErrors'=>new List<Account>{source}.deepClone()[0].hasErrors(),
            'mapErrors'=>new Map<String,Account>{'record'=>source}.deepClone().get('record').hasErrors(),
            'singleErrors'=>source.clone().hasErrors()};
    }
}
