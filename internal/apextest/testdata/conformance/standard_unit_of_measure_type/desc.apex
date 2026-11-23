Schema.DescribeFieldResult f = UnitOfMeasure.Type.getDescribe();
List<String> v = new List<String>(); for (Schema.PicklistEntry e : f.getPicklistValues()) v.add(e.getValue() + (e.isDefaultValue() ? '*' : ''));
System.debug('R5|uomdesc|nillable=' + f.isNillable() + '|defaultedOnCreate=' + f.isDefaultedOnCreate() + '|default=' + f.getDefaultValue() + '|values=' + String.join(v, ','));
Boolean hasIsActive = Schema.SObjectType.Account.fields.getMap().containsKey('IsActive');
System.debug('R5|acctIsActive|' + hasIsActive);
