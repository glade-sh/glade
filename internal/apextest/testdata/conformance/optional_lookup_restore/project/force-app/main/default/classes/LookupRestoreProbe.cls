public class LookupRestoreProbe {
    private static Map<String,Object> snapshot(List<Leaf__c> leaves, Id originalId, Id replacementId) {
        Set<Id> ids = new Map<Id,Leaf__c>(leaves).keySet();
        Map<Id,Leaf__c> stored = new Map<Id,Leaf__c>([SELECT Id,Name,Anchor__c FROM Leaf__c WHERE Id IN :ids WITH SYSTEM_MODE]);
        Map<String,Object> observed = new Map<String,Object>();
        List<String> labels = new List<String>{'unchanged','renamed','reparented','cleared'};
        for (Integer index=0; index<leaves.size(); index++) {
            Leaf__c leaf = stored.get(leaves[index].Id);
            String parent = leaf.Anchor__c == null ? 'null' : leaf.Anchor__c == originalId ? 'original' : leaf.Anchor__c == replacementId ? 'replacement' : 'other';
            observed.put(labels[index], parent);
        }
        observed.put('renamedName', stored.get(leaves[1].Id).Name);
        observed.put('count', stored.size());
        return observed;
    }

    public static Map<String,Object> observe() {
        Anchor__c original = new Anchor__c(Name='Owned original anchor');
        Anchor__c replacement = new Anchor__c(Name='Owned replacement anchor');
        Database.insert(new List<Anchor__c>{original,replacement},true,AccessLevel.SYSTEM_MODE);
        List<Leaf__c> leaves = new List<Leaf__c>{
            new Leaf__c(Name='Owned unchanged',Anchor__c=original.Id),
            new Leaf__c(Name='Owned renamed',Anchor__c=original.Id),
            new Leaf__c(Name='Owned reparented',Anchor__c=original.Id),
            new Leaf__c(Name='Owned cleared',Anchor__c=original.Id)
        };
        Database.insert(leaves,true,AccessLevel.SYSTEM_MODE);
        Map<String,Object> observed = new Map<String,Object>();
        observed.put('before',snapshot(leaves,original.Id,replacement.Id));
        Database.delete(new List<Anchor__c>{original},true,AccessLevel.SYSTEM_MODE);
        observed.put('deleted',snapshot(leaves,original.Id,replacement.Id));
        Database.update(new List<Leaf__c>{new Leaf__c(Id=leaves[1].Id,Name='Owned changed while deleted')},true,AccessLevel.SYSTEM_MODE);
        Database.update(new List<Leaf__c>{new Leaf__c(Id=leaves[2].Id,Anchor__c=replacement.Id)},true,AccessLevel.SYSTEM_MODE);
        Database.update(new List<Leaf__c>{new Leaf__c(Id=leaves[3].Id,Anchor__c=null)},true,AccessLevel.SYSTEM_MODE);
        observed.put('modifiedWhileDeleted',snapshot(leaves,original.Id,replacement.Id));
        Database.undelete(new List<Anchor__c>{original},true,AccessLevel.SYSTEM_MODE);
        observed.put('restored',snapshot(leaves,original.Id,replacement.Id));
        return observed;
    }
}
