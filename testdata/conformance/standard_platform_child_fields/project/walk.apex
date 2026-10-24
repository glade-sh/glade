        Set<String> rows = new Set<String>();
        List<String> failures = new List<String>();
        Map<String, Set<String>> selected = new Map<String, Set<String>>{
            'Account' => new Set<String>{'FinanceBalanceSnapshots', 'FinanceTransactions', 'MessagingEndUsers', 'ExtlRecShrStagedRecordAccounts', 'ParentRecords'},
            'Case' => new Set<String>{'MessagingSessions', 'ParentRecords'},
            'Contact' => new Set<String>{'MessagingEndUsers', 'ParentRecords'},
            'Opportunity' => new Set<String>{'MessagingSessions', 'Orders', 'ParentRecords'},
            'Lead' => new Set<String>{'ParentRecords'},
            'Product2' => new Set<String>{'ParentRecords'},
            'User' => new Set<String>{'MessagingEndUsers'}};
        for (Schema.SObjectType parentType : new List<Schema.SObjectType>{Lead.SObjectType, Product2.SObjectType, Account.SObjectType, Case.SObjectType, Contact.SObjectType, Opportunity.SObjectType, User.SObjectType}) {
            String parent = String.valueOf(parentType);
            for (Schema.ChildRelationship cr : parentType.getDescribe().getChildRelationships()) {
                if (String.isBlank(cr.getRelationshipName())) { continue; }
                try {
                    String obj = String.valueOf(cr.getChildSObject());
                    String fieldName = String.valueOf(cr.getField());
                    Schema.DescribeFieldResult d = cr.getField().getDescribe();
                    if (!selected.get(parent).contains(cr.getRelationshipName())) { continue; }
                    List<String> refs = new List<String>();
                    for (Schema.SObjectType r : d.getReferenceTo()) { refs.add(String.valueOf(r)); }
                    refs.sort();
                    rows.add(String.join(new List<String>{parent, cr.getRelationshipName(), obj, d.getName(), String.valueOf(d.getType()), String.join(refs, ','), String.valueOf(d.isNillable()), String.valueOf(d.isCreateable()), String.valueOf(d.isUpdateable()), String.valueOf(d.isDefaultedOnCreate()), d.getRelationshipName(), String.valueOf(d.isAccessible())}, '|'));
                } catch (Exception e) {
                    failures.add(parent + ':' + cr.getRelationshipName() + ':' + e.getMessage());
                }
            }
        }
        List<String> out = new List<String>(rows);
        out.sort();
        failures.sort();
        out.addAll(failures);
