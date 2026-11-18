Schema.DescribeFieldResult d = DescribeProbe.token().getDescribe();
System.debug('CROSS=' + d.getName() + ':' + d.getType() + ':' + d.getLength());
System.debug('FACETS=' + d.isAggregatable() + ':' + d.getPrecision() + ':' + d.getScale() + ':' + d.getSoapType() + ':' + d.isAutoNumber() + ':' + d.isCascadeDelete() + ':' + d.isCaseSensitive() + ':' + d.isCustom() + ':' + d.isDefaultedOnCreate() + ':' + d.isDependentPicklist() + ':' + d.isExternalId() + ':' + d.isIdLookup() + ':' + d.isNameField() + ':' + d.isUnique());
