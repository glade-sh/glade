public class FeatureParameterObservation {
    public static Map<String, Object> read() {
        Map<String, Object> row = new Map<String, Object>();
        row.put('boolean_default', System.FeatureManagement.checkPackageBooleanValue('Enabled'));
        row.put('boolean_case_insensitive', System.FeatureManagement.checkPackageBooleanValue('eNaBlEd'));
        row.put('integer_default', System.FeatureManagement.checkPackageIntegerValue('Quota'));
        row.put('date_default', System.FeatureManagement.checkPackageDateValue('StartDate'));
        row.put('locked_default', System.FeatureManagement.checkPackageBooleanValue('Locked'));
        row.put('missing_boolean_check', missingBooleanCheck('Missing'));
        row.put('missing_integer_check', missingIntegerCheck('Missing'));
        row.put('missing_date_check', missingDateCheck('Missing'));
        row.put('missing_integer_set', missingIntegerSet('Missing'));
        row.put('wrong_type_check', missingIntegerCheck('Enabled'));
        row.put('wrong_type_set', missingIntegerSet('Enabled'));
        return row;
    }

    private static String missingBooleanCheck(String name) {
        try {
            System.FeatureManagement.checkPackageBooleanValue(name);
            return 'returned';
        } catch (Exception e) {
            return e.getTypeName() + ': ' + e.getMessage();
        }
    }

    private static String missingIntegerCheck(String name) {
        try {
            System.FeatureManagement.checkPackageIntegerValue(name);
            return 'returned';
        } catch (Exception e) {
            return e.getTypeName() + ': ' + e.getMessage();
        }
    }

    private static String missingDateCheck(String name) {
        try {
            System.FeatureManagement.checkPackageDateValue(name);
            return 'returned';
        } catch (Exception e) {
            return e.getTypeName() + ': ' + e.getMessage();
        }
    }

    private static String missingIntegerSet(String name) {
        try {
            System.FeatureManagement.setPackageIntegerValue(name, 7);
            return 'returned';
        } catch (Exception e) {
            return e.getTypeName() + ': ' + e.getMessage();
        }
    }
}
