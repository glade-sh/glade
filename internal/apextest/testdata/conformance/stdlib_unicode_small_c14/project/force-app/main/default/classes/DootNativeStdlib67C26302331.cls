@IsTest
private class DootNativeStdlib67C26302331 {
    @IsTest static void baseline() {
        System.assertEquals(2, 1 + 1, 'BASELINE|arithmetic');
    }

    private static Map<String, Object> envelope() {
        return new Map<String, Object>{
            'sourceApi' => '67.0', 'route' => 'named-test-check-only',
            'locale' => UserInfo.getLocale(), 'language' => UserInfo.getLanguage(),
            'timeZone' => UserInfo.getTimeZone().getID()
        };
    }

    private static void recordException(Map<String, Object> row, String stage, Exception e) {
        row.put('status', 'exception');
        row.put('stage', stage);
        row.put('exceptionType', e.getTypeName());
        row.put('exceptionMessage', e.getMessage());
    }

    private static Map<String, Object> decimalLexeme(String lexeme) {
        Map<String, Object> row = new Map<String, Object>{'input' => lexeme};
        String stage = 'valueOf';
        try {
            Decimal value = Decimal.valueOf(lexeme);
            row.put('value', value);
            stage = 'toPlainString';
            row.put('plain', value.toPlainString());
            stage = 'scale';
            row.put('scale', value.scale());
            row.put('status', 'ok');
        } catch (Exception e) {
            recordException(row, stage, e);
        }
        return row;
    }

    @IsTest static void decimalHex() {
        Map<String, Object> data = envelope();
        data.put('hex', decimalLexeme('0x1p0'));
        data.put('one', decimalLexeme('1'));
        data.put('fraction', decimalLexeme('1.25'));
        data.put('invalid', decimalLexeme('not-a-decimal'));
        System.assert(false, 'CAPTURE|decimalhex|' + JSON.serialize(data));
    }

    private static Map<String, Object> accountPair(String variant) {
        Map<String, Object> row = new Map<String, Object>();
        String stage = 'fixture';
        try {
            Account a = new Account(Id='001000000000001', Name='A');
            Account b = new Account(Id='001000000000001', Name='B');
            if (variant == 'equal') {
                b = new Account(Id='001000000000001', Name='A');
            } else if (variant == 'distinctId') {
                b = new Account(Id='001000000000002', Name='A');
            }
            Map<Account, Integer> values = new Map<Account, Integer>();
            stage = 'equals';
            row.put('equal', a.equals(b));
            stage = 'firstPutAndLookups';
            values.put(a, 1);
            row.put('afterFirst', new Map<String, Object>{
                'size' => values.size(), 'a' => values.get(a), 'b' => values.get(b)
            });
            stage = 'secondPutAndLookups';
            values.put(b, 2);
            row.put('afterSecond', new Map<String, Object>{
                'size' => values.size(), 'a' => values.get(a), 'b' => values.get(b)
            });
            row.put('status', 'ok');
        } catch (Exception e) {
            recordException(row, stage, e);
        }
        return row;
    }

    @IsTest static void sobjectKeys() {
        Map<String, Object> data = envelope();
        data.put('sameIdDifferentName', accountPair('candidate'));
        data.put('equalFields', accountPair('equal'));
        data.put('distinctIdSameName', accountPair('distinctId'));
        System.assert(false, 'CAPTURE|sobjectkeys|' + JSON.serialize(data));
    }

    private static Map<String, Object> listPair(String variant) {
        Map<String, Object> row = new Map<String, Object>();
        String stage = 'fixture';
        try {
            List<String> a = new List<String>{'a, b'};
            List<String> b = new List<String>{'a', 'b'};
            if (variant == 'equal') {
                a = new List<String>{'a', 'b'};
                b = new List<String>{'a', 'b'};
            } else if (variant == 'ordinaryUnequal') {
                a = new List<String>{'a'};
                b = new List<String>{'b'};
            }
            Map<List<String>, Integer> values = new Map<List<String>, Integer>();
            stage = 'equals';
            row.put('equal', a.equals(b));
            stage = 'firstPutAndLookups';
            values.put(a, 1);
            row.put('afterFirst', new Map<String, Object>{
                'size' => values.size(), 'a' => values.get(a), 'b' => values.get(b)
            });
            stage = 'secondPutAndLookups';
            values.put(b, 2);
            row.put('afterSecond', new Map<String, Object>{
                'size' => values.size(), 'a' => values.get(a), 'b' => values.get(b)
            });
            row.put('status', 'ok');
        } catch (Exception e) {
            recordException(row, stage, e);
        }
        return row;
    }

    @IsTest static void listKeys() {
        Map<String, Object> data = envelope();
        data.put('ambiguousDisplay', listPair('candidate'));
        data.put('equalLists', listPair('equal'));
        data.put('ordinaryUnequal', listPair('ordinaryUnequal'));
        System.assert(false, 'CAPTURE|listkeys|' + JSON.serialize(data));
    }

    private static Map<String, Object> setPair(String variant) {
        Map<String, Object> row = new Map<String, Object>();
        String stage = 'fixture';
        try {
            Set<String> a = new Set<String>();
            Set<String> b = new Set<String>();
            a.add('a');
            a.add('b');
            if (variant == 'sameHistory') {
                b.add('a');
                b.add('b');
            } else if (variant == 'unequal') {
                b.add('a');
                b.add('c');
            } else {
                b.add('b');
                b.add('a');
            }
            Map<Set<String>, Integer> values = new Map<Set<String>, Integer>();
            stage = 'equals';
            row.put('equal', a.equals(b));
            stage = 'firstPutAndLookups';
            values.put(a, 1);
            row.put('afterFirst', new Map<String, Object>{
                'size' => values.size(), 'a' => values.get(a), 'b' => values.get(b)
            });
            stage = 'secondPutAndLookups';
            values.put(b, 2);
            row.put('afterSecond', new Map<String, Object>{
                'size' => values.size(), 'a' => values.get(a), 'b' => values.get(b)
            });
            row.put('status', 'ok');
        } catch (Exception e) {
            recordException(row, stage, e);
        }
        return row;
    }

    @IsTest static void setKeys() {
        Map<String, Object> data = envelope();
        data.put('oppositeInsertion', setPair('candidate'));
        data.put('sameInsertion', setPair('sameHistory'));
        data.put('unequalMembers', setPair('unequal'));
        System.assert(false, 'CAPTURE|setkeys|' + JSON.serialize(data));
    }

    private static Map<String, Object> stringOperation(String operation, String text, String needle) {
        Map<String, Object> row = new Map<String, Object>();
        try {
            if (operation == 'equalsIgnoreCase') {
                row.put('value', text.equalsIgnoreCase(needle));
            } else if (operation == 'containsIgnoreCase') {
                row.put('value', text.containsIgnoreCase(needle));
            } else if (operation == 'indexOfIgnoreCase') {
                row.put('value', text.indexOfIgnoreCase(needle));
            } else if (operation == 'startsWithIgnoreCase') {
                row.put('value', text.startsWithIgnoreCase(needle));
            } else if (operation == 'endsWithIgnoreCase') {
                row.put('value', text.endsWithIgnoreCase(needle));
            } else {
                String result = text.removeStartIgnoreCase(needle);
                row.put('value', result);
                row.put('utf16', result.getChars());
                row.put('length', result.length());
            }
            row.put('status', 'ok');
        } catch (Exception e) {
            recordException(row, operation, e);
        }
        return row;
    }

    private static Map<String, Object> searchTuple(String text, String needle) {
        return new Map<String, Object>{
            'equals' => stringOperation('equalsIgnoreCase', text, needle),
            'contains' => stringOperation('containsIgnoreCase', text, needle),
            'index' => stringOperation('indexOfIgnoreCase', text, needle)
        };
    }

    @IsTest static void unicodeSigma() {
        Map<String, Object> data = envelope();
        data.put('sigmaFinalSigma', searchTuple('\u03A3', '\u03C2'));
        data.put('reverseSigma', searchTuple('\u03C2', '\u03A3'));
        data.put('asciiKk', searchTuple('K', 'k'));
        data.put('unrelated', searchTuple('\u03A3', 'x'));
        System.assert(false, 'CAPTURE|sigma|' + JSON.serialize(data));
    }

    private static Map<String, Object> boundaryTuple(String text, String needle) {
        return new Map<String, Object>{
            'equals' => stringOperation('equalsIgnoreCase', text, needle),
            'starts' => stringOperation('startsWithIgnoreCase', text + 'x', needle),
            'ends' => stringOperation('endsWithIgnoreCase', 'x' + text, needle),
            'removeStart' => stringOperation('removeStartIgnoreCase', text + 'x', needle),
            'wholeStarts' => stringOperation('startsWithIgnoreCase', text, needle),
            'wholeEnds' => stringOperation('endsWithIgnoreCase', text, needle)
        };
    }

    @IsTest static void unicodeKelvin() {
        Map<String, Object> data = envelope();
        data.put('kelvinToAscii', boundaryTuple('\u212A', 'k'));
        data.put('asciiToKelvin', boundaryTuple('k', '\u212A'));
        data.put('asciiKk', boundaryTuple('K', 'k'));
        data.put('unrelated', boundaryTuple('\u212A', 'z'));
        System.assert(false, 'CAPTURE|kelvin|' + JSON.serialize(data));
    }
}
