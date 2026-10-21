@IsTest
private class PlatformTokenAssertTest {
    private static String assertionOutcome(Object value, Type expected) {
        Assert.isInstanceOfType(value, expected);
        return 'pass';
    }

    private static void observe(String method, Map<String, Object> observation) {
        System.assert(false, 'OBSERVED|' + method + '|' + JSON.serialize(observation));
    }

    @IsTest static void selectOptionNames() {
        observe('selectOptionNames', new Map<String, Object>{
            'bare' => SelectOption.class.getName(),
            'qualified' => System.SelectOption.class.getName(),
            'constructed' => assertionOutcome(new SelectOption('one', 'First'), SelectOption.class)
        });
    }

    @IsTest static void locationAssertions() {
        Object position = System.Location.newInstance(12.5, -37.25);
        Object textControl = 'ordinary text';
        observe('locationAssertions', new Map<String, Object>{
            'bareName' => Location.class.getName(),
            'qualifiedName' => System.Location.class.getName(),
            'bareAssert' => assertionOutcome(position, Location.class),
            'qualifiedAssert' => assertionOutcome(position, System.Location.class),
            'bareInstanceof' => position instanceof Location,
            'qualifiedInstanceof' => position instanceof System.Location,
            'stringControl' => textControl instanceof System.Location
        });
    }

    @IsTest static void idAssertions() {
        Object boxedString = '001000000000001';
        Object boxedId = (Id)'001000000000001';
        Object invalidString = 'ordinary text';
        Object defaultRecordTypeId = Account.SObjectType.getDescribe().getRecordTypeInfos()[0].getRecordTypeId();
        observe('idAssertions', new Map<String, Object>{
            'typedIdAssert' => assertionOutcome(boxedId, Id.class),
            'stringIdInstanceof' => boxedString instanceof Id,
            'typedIdInstanceof' => boxedId instanceof Id,
            'invalidIdInstanceof' => invalidString instanceof Id,
            'typedIdStringAssert' => assertionOutcome(boxedId, String.class),
            'defaultIdPresent' => defaultRecordTypeId != null,
            'defaultIdInstanceof' => defaultRecordTypeId instanceof Id,
            'defaultIdAssert' => defaultRecordTypeId == null ? 'null' : assertionOutcome(defaultRecordTypeId, Id.class)
        });
    }
}
