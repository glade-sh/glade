public class ParentProjectionProbe {
    private static List<String> keys(SObject record) {
        List<String> result = new List<String>(record.getPopulatedFieldsAsMap().keySet());
        result.sort();
        return result;
    }

    private static Map<String,Object> shape(Contact child, Id expectedParent) {
        Map<String,Object> observed = new Map<String,Object>{'keys'=>keys(child)};
        try {
            observed.put('foreignKey', child.AccountId == expectedParent);
            observed.put('foreignKeyNull', child.AccountId == null);
        } catch (Exception issue) {
            observed.put('foreignKeyError', issue.getTypeName() + ':' + issue.getMessage());
        }
        try {
            Account parent = child.Account;
            observed.put('parentNull', parent == null);
            if (parent != null) {
                observed.put('parentKeys', keys(parent));
                observed.put('sameParent', parent.Id == expectedParent);
            }
        } catch (Exception issue) {
            observed.put('parentError', issue.getTypeName() + ':' + issue.getMessage());
        }
        return observed;
    }

    public static Map<String,Object> observe() {
        Account parent = new Account(Name='Owned projected parent');
        insert parent;
        Contact linked = new Contact(LastName='Owned linked child', AccountId=parent.Id);
        Contact unlinked = new Contact(LastName='Owned unlinked child');
        insert new List<Contact>{linked, unlinked};
        Map<String,Object> observed = new Map<String,Object>();

        Contact byParentId = [SELECT Id, LastName, Account.Id FROM Contact WHERE Id=:linked.Id];
        observed.put('parentId', shape(byParentId, parent.Id));
        Contact byParentName = [SELECT Id, LastName, Account.Name FROM Contact WHERE Id=:linked.Id];
        observed.put('parentName', shape(byParentName, parent.Id));
        Contact nested = [SELECT Id, Account.Owner.Id FROM Contact WHERE Id=:linked.Id];
        Map<String,Object> nestedShape = shape(nested, parent.Id);
        try {
            nestedShape.put('ownerForeignKey', nested.Account.OwnerId == UserInfo.getUserId());
            nestedShape.put('ownerForeignKeyNull', nested.Account.OwnerId == null);
        } catch (Exception issue) {
            nestedShape.put('ownerForeignKeyError', issue.getTypeName() + ':' + issue.getMessage());
        }
        nestedShape.put('ownerKeys', keys(nested.Account.Owner));
        observed.put('nested', nestedShape);
        Contact explicitForeignKey = [SELECT Id, AccountId, Account.Name FROM Contact WHERE Id=:linked.Id];
        observed.put('explicitForeignKey', shape(explicitForeignKey, parent.Id));
        Contact nullParent = [SELECT Id, Account.Name FROM Contact WHERE Id=:unlinked.Id];
        observed.put('nullParent', shape(nullParent, null));

        Account loaded = byParentName.Account;
        loaded.Name = 'Owned local mutation';
        observed.put('parentAliasMutation', byParentName.Account.Name);
        observed.put('persistedParentName', [SELECT Name FROM Account WHERE Id=:parent.Id].Name);

        Map<String,Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
        observed.put('cleanStatusPresent', fields.containsKey('CleanStatus'));
        if (fields.containsKey('CleanStatus')) {
            Schema.DescribeFieldResult description = fields.get('CleanStatus').getDescribe();
            observed.put('cleanStatusType', String.valueOf(description.getType()));
            observed.put('cleanStatusUpdateable', description.isUpdateable());
            observed.put('cleanStatusCreateable', description.isCreateable());
            observed.put('cleanStatusNillable', description.isNillable());
            observed.put('cleanStatusDefaultedOnCreate', description.isDefaultedOnCreate());
            observed.put('cleanStatusDefaultValue', description.getDefaultValue());
            observed.put('cleanStatusLength', description.getLength());
            List<Map<String,Object>> choices = new List<Map<String,Object>>();
            for (Schema.PicklistEntry entry : description.getPicklistValues()) {
                if (entry.isActive()) {
                    choices.add(new Map<String,Object>{
                        'value'=>entry.getValue(), 'label'=>entry.getLabel(),
                        'default'=>entry.isDefaultValue()
                    });
                }
            }
            observed.put('cleanStatusValues', choices);
        }
        try {
            List<SObject> result = Database.query('SELECT Id,CleanStatus FROM Account WHERE Id=\'' + parent.Id + '\'');
            observed.put('cleanStatusQuery', 'accepted');
            observed.put('cleanStatusRows', result.size());
            observed.put('cleanStatusValue', result[0].get('CleanStatus'));
        } catch (Exception issue) {
            observed.put('cleanStatusQuery', issue.getTypeName() + ':' + issue.getMessage());
        }
        return observed;
    }
}
