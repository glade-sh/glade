Schema.DescribeFieldResult d = DescribeProbe.token().getDescribe();
System.debug('D=' + d.getLocalName() + ':' + d.getCompoundFieldName() + ':' + d.getReferenceTargetField() + ':' + d.isRestrictedPicklist());
System.debug('E=' + d.isAiPredictionField() + ':' + d.isDisplayLocationInDecimal() + ':' + d.isFormulaTreatNullNumberAsZero());
