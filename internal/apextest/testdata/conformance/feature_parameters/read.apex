System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(new Map<String, Object>{
    'boolean_after_set' => System.FeatureManagement.checkPackageBooleanValue('Enabled'),
    'integer_after_set' => System.FeatureManagement.checkPackageIntegerValue('Quota'),
    'date_after_set' => System.FeatureManagement.checkPackageDateValue('StartDate')
}));
