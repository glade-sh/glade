List<String> rows = new List<String>();
        for (Schema.ChildRelationship child : LinkProbe__c.SObjectType.getDescribe().getChildRelationships()) {
            String name = child.getRelationshipName();
            if (name != null) {
                rows.add(name + '|' + String.valueOf(child.getChildSObject()) + '|' + String.valueOf(child.getField()) + '|' + child.isCascadeDelete() + '|' + child.isRestrictedDelete());
            }
        }
        rows.sort();
        System.debug(LoggingLevel.ERROR, 'DESCRIBE_ROWS|' + JSON.serialize(rows));
