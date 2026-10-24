@IsTest
private class FeatureParameterSetterRouteTest {
    @IsTest
    static void setValues() {
        System.assertEquals(true, System.FeatureManagement.checkPackageBooleanValue('Enabled'));
        System.assertEquals(11, System.FeatureManagement.checkPackageIntegerValue('Quota'));
        System.assertEquals(Date.newInstance(2020, 2, 2), System.FeatureManagement.checkPackageDateValue('StartDate'));
        System.FeatureManagement.setPackageBooleanValue('eNaBlEd', false);
        System.FeatureManagement.setPackageIntegerValue('qUoTa', 17);
        System.FeatureManagement.setPackageDateValue('sTaRtDaTe', Date.newInstance(2020, 3, 4));
        System.assert(false, 'OBS|' + JSON.serialize(new Map<String, Object>{
            'boolean_after_set' => System.FeatureManagement.checkPackageBooleanValue('ENABLED'),
            'integer_after_set' => System.FeatureManagement.checkPackageIntegerValue('QUOTA'),
            'date_after_set' => System.FeatureManagement.checkPackageDateValue('STARTDATE')
        }));
    }

    @IsTest
    static void defaults() {
        System.assert(false, 'OBS|' + JSON.serialize(new Map<String, Object>{
            'boolean_default' => System.FeatureManagement.checkPackageBooleanValue('Enabled'),
            'integer_default' => System.FeatureManagement.checkPackageIntegerValue('Quota'),
            'date_default' => System.FeatureManagement.checkPackageDateValue('StartDate')
        }));
    }

    @IsTest
    static void nullSet() {
        System.FeatureManagement.setPackageIntegerValue(null, 7);
    }

    @IsTest
    static void lockedSet() {
        Boolean before = System.FeatureManagement.checkPackageBooleanValue('Locked');
        System.FeatureManagement.setPackageBooleanValue('Locked', true);
        System.assert(false, 'OBS|' + JSON.serialize(new Map<String, Object>{
            'locked_before_set' => before,
            'locked_after_set' => System.FeatureManagement.checkPackageBooleanValue('Locked')
        }));
    }
}
