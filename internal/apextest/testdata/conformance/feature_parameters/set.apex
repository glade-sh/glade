System.FeatureManagement.setPackageBooleanValue('eNaBlEd', false);
System.FeatureManagement.setPackageIntegerValue('qUoTa', 17);
System.FeatureManagement.setPackageDateValue('sTaRtDaTe', Date.newInstance(2020, 3, 4));
System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(new Map<String, Object>{
    'boolean_after_set' => System.FeatureManagement.checkPackageBooleanValue('ENABLED'),
    'integer_after_set' => System.FeatureManagement.checkPackageIntegerValue('QUOTA'),
    'date_after_set' => System.FeatureManagement.checkPackageDateValue('STARTDATE')
}));
