Schema.DescribeFieldResult d = DescribeProbe.token().getDescribe();
System.debug('A=' + d.getByteLength() + ':' + d.getDigits() + ':' + d.getCalculatedFormula() + ':' + d.isHtmlFormatted() + ':' + d.isEncrypted() + ':' + d.getRelationshipName() + ':' + d.getReferenceTo().size() + ':' + d.getPicklistValues().size());
System.debug('B=' + d.getInlineHelpText() + ':' + d.getController() + ':' + d.getRelationshipOrder() + ':' + d.getDefaultValue() + ':' + d.getDefaultValueFormula());
System.debug('C=' + d.isCascadeDelete() + ':' + d.isDeprecatedAndHidden() + ':' + d.isHighScaleNumber() + ':' + d.isNamePointing() + ':' + d.isQueryByDistance() + ':' + d.isRestrictedDelete() + ':' + d.isSearchPrefilterable() + ':' + d.isWriteRequiresMasterRead());
