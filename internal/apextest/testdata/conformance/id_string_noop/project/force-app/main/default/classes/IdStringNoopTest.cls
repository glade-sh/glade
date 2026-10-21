@IsTest
private class IdStringNoopTest {
    private static Map<String, Object> describe(Object value) {
        Map<String, Object> description = new Map<String, Object>{
            'idInstanceof' => value instanceof Id,
            'stringInstanceof' => value instanceof String
        };
        try {
            SelectOption option = (SelectOption)value;
            description.put('cast', 'accepted');
        } catch (TypeException error) {
            description.put('cast', error.getMessage());
        }
        return description;
    }

    private static void observe(String method, Map<String, Object> observation) {
        System.assert(false, 'OBSERVED|' + method + '|' + JSON.serialize(observation));
    }

    @IsTest static void noopIdOperations() {
        Object identifier = (Id)'001000000000001';
        observe('noopIdOperations', new Map<String, Object>{
            'original' => describe(identifier),
            'castString' => describe((String)identifier),
            'replace' => describe(((String)identifier).replace('absent-marker', 'x')),
            'replaceAll' => describe(((String)identifier).replaceAll('absent-marker', 'x'))
        });
    }

    @IsTest static void changedIdOperations() {
        Object identifier = (Id)'001000000000001';
        observe('changedIdOperations', new Map<String, Object>{
            'replace' => describe(((String)identifier).replace('0', '-')),
            'replaceAll' => describe(((String)identifier).replaceAll('0', '-'))
        });
    }

    @IsTest static void ordinaryStringOperations() {
        Object text = '001000000000001';
        observe('ordinaryStringOperations', new Map<String, Object>{
            'original' => describe(text),
            'replace' => describe(((String)text).replace('absent-marker', 'x')),
            'replaceAll' => describe(((String)text).replaceAll('absent-marker', 'x'))
        });
    }

    @IsTest static void rawStringIdAssertion() {
        Object identifier = (Id)'001000000000001';
        Object replaced = ((String)identifier).replace('$INDEX', '0');
        Assert.isInstanceOfType(replaced, Id.class);
        observe('rawStringIdAssertion', new Map<String, Object>{'unexpectedlyAccepted' => true});
    }

    @IsTest static void rawOrdinary15Assertion() {
        Object text = '001000000000001';
        Assert.isInstanceOfType(text, Id.class);
        observe('rawOrdinary15Assertion', new Map<String, Object>{'accepted' => true});
    }

    @IsTest static void rawOrdinary18Assertion() {
        Object text = '001000000000001AAA';
        Assert.isInstanceOfType(text, Id.class);
        observe('rawOrdinary18Assertion', new Map<String, Object>{'accepted' => true});
    }

    @IsTest static void rawCastIdStringAssertion() {
        Object identifier = (Id)'001000000000001';
        Object text = (String)identifier;
        Assert.isInstanceOfType(text, Id.class);
        observe('rawCastIdStringAssertion', new Map<String, Object>{'accepted' => true});
    }
}
