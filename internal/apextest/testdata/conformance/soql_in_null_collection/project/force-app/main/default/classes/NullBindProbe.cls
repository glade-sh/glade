public class NullBindProbe {
    static List<Id> nothing() { return null; }
    static Set<Id> nothingSet() { return null; }
    public List<Id> heldList;
    public Set<Id> heldSet;
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        insert new Account(Name = 'NullBind Seed');
        List<Id> nullIds = null;
        Set<Id> nullIdSet = null;
        List<String> nullNames = null;
        Set<String> nullNameSet = null;
        List<Id> emptyIds = new List<Id>();
        String nullName = null;
        try { result.put('listIdIn', [SELECT Id FROM Account WHERE Id IN :nullIds].size()); } catch (Exception issue) { result.put('listIdIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('setIdIn', [SELECT Id FROM Account WHERE Id IN :nullIdSet].size()); } catch (Exception issue) { result.put('setIdIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('listNameIn', [SELECT Id FROM Account WHERE Name IN :nullNames].size()); } catch (Exception issue) { result.put('listNameIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('setNameIn', [SELECT Id FROM Account WHERE Name IN :nullNameSet].size()); } catch (Exception issue) { result.put('setNameIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('orIn', [SELECT Id FROM Account WHERE Id IN :nullIds OR Name = 'NullBind Seed'].size()); } catch (Exception issue) { result.put('orIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('notIn', [SELECT Id FROM Account WHERE Id NOT IN :nullIds].size()); } catch (Exception issue) { result.put('notIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('countIn', [SELECT COUNT() FROM Account WHERE Id IN :nullIds]); } catch (Exception issue) { result.put('countIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('emptyIn', [SELECT Id FROM Account WHERE Id IN :emptyIds].size()); } catch (Exception issue) { result.put('emptyIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('equalsNull', [SELECT Id FROM Account WHERE Name = :nullName].size()); } catch (Exception issue) { result.put('equalsNull', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('dynamicIn', Database.query('SELECT Id FROM Account WHERE Id IN :nullIds').size()); } catch (Exception issue) { result.put('dynamicIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('bindMapIn', Database.queryWithBinds('SELECT Id FROM Account WHERE Id IN :ids', new Map<String,Object>{ 'ids' => nullIds }, AccessLevel.SYSTEM_MODE).size()); } catch (Exception issue) { result.put('bindMapIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('nestedIn', [SELECT Id, (SELECT Id FROM Contacts WHERE Id IN :nullIds) FROM Account WHERE Name = 'NullBind Seed'].size()); } catch (Exception issue) { result.put('nestedIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('dynamicSetIn', Database.query('SELECT Id FROM Account WHERE Id IN :nullIdSet').size()); } catch (Exception issue) { result.put('dynamicSetIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('bindMapSetIn', Database.queryWithBinds('SELECT Id FROM Account WHERE Id IN :ids', new Map<String,Object>{ 'ids' => nullIdSet }, AccessLevel.SYSTEM_MODE).size()); } catch (Exception issue) { result.put('bindMapSetIn', issue.getTypeName() + ':' + issue.getMessage()); }
        List<Id> uninitialized;
        try { result.put('uninitializedIn', [SELECT Id FROM Account WHERE Id IN :uninitialized].size()); } catch (Exception issue) { result.put('uninitializedIn', issue.getTypeName() + ':' + issue.getMessage()); }
        NullBindProbe holder = new NullBindProbe();
        try { result.put('fieldListIn', [SELECT Id FROM Account WHERE Id IN :holder.heldList].size()); } catch (Exception issue) { result.put('fieldListIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('fieldSetIn', [SELECT Id FROM Account WHERE Id IN :holder.heldSet].size()); } catch (Exception issue) { result.put('fieldSetIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('methodListIn', [SELECT Id FROM Account WHERE Id IN :nothing()].size()); } catch (Exception issue) { result.put('methodListIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('methodSetIn', [SELECT Id FROM Account WHERE Id IN :nothingSet()].size()); } catch (Exception issue) { result.put('methodSetIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('parentIn', [SELECT Id FROM Contact WHERE AccountId IN :nullIds].size()); } catch (Exception issue) { result.put('parentIn', issue.getTypeName() + ':' + issue.getMessage()); }
        try { result.put('stringSetIn', [SELECT Id FROM Account WHERE Name IN :nullNameSet OR Id IN :nullIdSet].size()); } catch (Exception issue) { result.put('stringSetIn', issue.getTypeName() + ':' + issue.getMessage()); }
        return result;
    }
}
