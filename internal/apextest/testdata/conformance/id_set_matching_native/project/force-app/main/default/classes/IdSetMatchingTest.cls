@IsTest private class IdSetMatchingTest {
    private static Object idContains(Set<Id> values, String needle) {
        try { return values.contains(needle); }
        catch (Exception ex) { return ex.getTypeName()+':'+ex.getMessage(); }
    }

    @IsTest static void membershipControls() {
        Id recordId = '001000000000071';
        String full = String.valueOf(recordId);
        String shortValue = new ApexPages.StandardController(new Account(Id=recordId)).getId();
        Set<Id> ids = new Set<Id>{recordId};
        Set<String> fullStrings = new Set<String>{full};
        Set<String> shortStrings = new Set<String>{shortValue};
        System.assert(false, 'OBSERVED|membershipControls|'+JSON.serialize(new Map<String,Object>{
            'shortString'=>idContains(ids, shortValue), 'fullString'=>idContains(ids, full),
            'id'=>ids.contains(recordId), 'null'=>idContains(ids, null),
            'invalid'=>idContains(ids, 'invalid'), 'wrongLength'=>idContains(ids, '00100000000007'),
            'fullStringsId'=>fullStrings.contains(recordId),
            'shortStringsId'=>shortStrings.contains(recordId),
            'fullStringsShort'=>fullStrings.contains(shortValue),
            'shortStringsFull'=>shortStrings.contains(full),
            'fullStringsFull'=>fullStrings.contains(full),
            'shortStringsShort'=>shortStrings.contains(shortValue)
        }));
    }

    private static Object boxAmount(Decimal value) { return value; }

    @IsTest static void stubDecimalArgument() {
        IdMatchProvider provider = new IdMatchProvider();
        IdMatchPort port = (IdMatchPort)Test.createStub(IdMatchPort.class, provider);
        port.acceptAmount(10);
        Object literal = provider.actual;
        Decimal amount = 10;
        port.acceptAmount(amount);
        Object typed = provider.actual;
        Object ordinary = boxAmount(10);
        System.assert(false, 'OBSERVED|stubDecimalArgument|'+JSON.serialize(new Map<String,Object>{
            'literalEqualsTyped'=>new List<Object>{literal}==new List<Object>{typed},
            'literalEqualsOrdinary'=>new List<Object>{literal}==new List<Object>{ordinary},
            'typedEqualsOrdinary'=>new List<Object>{typed}==new List<Object>{ordinary},
            'literalEqualsInteger'=>new List<Object>{literal}==new List<Object>{10},
            'signature'=>provider.signature[0].getName()
        }));
    }

    @IsTest static void directAndErased() {
        Id recordId = '001000000000071';
        String full = String.valueOf(recordId);
        String shortValue = new ApexPages.StandardController(new Account(Id=recordId)).getId();
        Set<Id> left = new Set<Id>{full};
        Set<Id> right = new Set<Id>{shortValue};
        Object first = left, second = right;
        System.assert(false, 'OBSERVED|directAndErased|'+JSON.serialize(new Map<String,Object>{
            'operator'=>left==right, 'method'=>left.equals(right),
            'erasedOperator'=>first==second, 'erasedMethod'=>first.equals(second),
            'nestedList'=>new List<Object>{first}==new List<Object>{second},
            'contains'=>left.contains(shortValue), 'reverseContains'=>right.contains(full),
            'leftSize'=>left.size(), 'rightSize'=>right.size(),
            'shortLength'=>shortValue.length(), 'fullLength'=>full.length(),
            'stringOperator'=>new Set<String>{full}==new Set<String>{shortValue}
        }));
    }

    @IsTest static void stubSetArgument() {
        Id recordId = '001000000000071';
        String full = String.valueOf(recordId);
        String shortValue = new ApexPages.StandardController(new Account(Id=recordId)).getId();
        IdMatchProvider provider = new IdMatchProvider();
        IdMatchPort port = (IdMatchPort)Test.createStub(IdMatchPort.class, provider);
        Set<Id> expected = new Set<Id>{full};
        provider.expected = expected;
        port.accept(new Set<Id>{shortValue});
        Set<Id> actual = (Set<Id>)provider.actual;
        System.assert(false, 'OBSERVED|stubSetArgument|'+JSON.serialize(new Map<String,Object>{
            'operator'=>expected==actual, 'erasedOperator'=>provider.expected==provider.actual,
            'erasedMethod'=>provider.expected.equals(provider.actual),
            'nestedList'=>new List<Object>{provider.expected}==new List<Object>{provider.actual},
            'contains'=>expected.contains(actual.iterator().next()),
            'signature'=>provider.signature[0].getName()
        }));
    }

    @IsTest static void stubReferenceArgument() {
        IdMatchProvider provider = new IdMatchProvider();
        IdMatchPort port = (IdMatchPort)Test.createStub(IdMatchPort.class, provider);
        IdMatchToken original = (IdMatchToken)Test.createStub(IdMatchToken.class, provider);
        IdMatchFactory.setToken(original);
        provider.expected = original;
        port.acceptToken(IdMatchFactory.getToken());
        System.assert(false, 'OBSERVED|stubReferenceArgument|'+JSON.serialize(new Map<String,Object>{
            'operator'=>provider.expected==provider.actual,
            'identity'=>provider.expected===provider.actual,
            'factoryIdentity'=>original===IdMatchFactory.getToken(),
            'nestedList'=>new List<Object>{provider.expected}==new List<Object>{provider.actual},
            'signature'=>provider.signature[0].getName()
        }));
    }
}
