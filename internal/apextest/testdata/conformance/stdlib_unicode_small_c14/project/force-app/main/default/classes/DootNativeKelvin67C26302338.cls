@IsTest
private class DootNativeKelvin67C26302338 {
    @IsTest static void baseline() {
        System.assertEquals(2, 1 + 1, 'BASELINE|arithmetic');
    }

    private static Object booleanValue(String operation, String text, String needle) {
        try {
            if (operation == 'eq') {
                return text.equalsIgnoreCase(needle);
            } else if (operation == 'start') {
                return text.startsWithIgnoreCase(needle);
            } else {
                return text.endsWithIgnoreCase(needle);
            }
        } catch (Exception e) {
            return new Map<String, Object>{
                'exceptionType' => e.getTypeName(), 'exceptionMessage' => e.getMessage()
            };
        }
    }

    private static Map<String, Object> removal(String text, String needle) {
        Map<String, Object> row = new Map<String, Object>();
        String stage = 'removeStartIgnoreCase';
        try {
            String result = text.removeStartIgnoreCase(needle);
            row.put('text', result);
            stage = 'getChars';
            row.put('utf16', result.getChars());
            stage = 'length';
            row.put('length', result.length());
        } catch (Exception e) {
            row.put('stage', stage);
            row.put('exceptionType', e.getTypeName());
            row.put('exceptionMessage', e.getMessage());
        }
        return row;
    }

    private static Map<String, Object> tuple(String text, String needle) {
        return new Map<String, Object>{
            'eq' => booleanValue('eq', text, needle),
            'start' => booleanValue('start', text + 'x', needle),
            'end' => booleanValue('end', 'x' + text, needle),
            'wholeStart' => booleanValue('start', text, needle),
            'wholeEnd' => booleanValue('end', text, needle),
            'remove' => removal(text + 'x', needle)
        };
    }

    private static String payload(Map<String, Object> data) {
        String serialized = JSON.serialize(data);
        if (serialized.length() > 500) {
            // Never cut a JSON object or treat its partial fields as an answer.
            return JSON.serialize(new Map<String, Object>{
                'status' => 'inconclusive_payload_too_long', 'chars' => serialized.length()
            });
        }
        return serialized;
    }

    @IsTest static void kelvinToAscii() {
        Map<String, Object> data = tuple('\u212A', 'k');
        System.assert(false, 'CAPTURE|kelvinToAscii|' + payload(data));
    }

    @IsTest static void asciiToKelvin() {
        Map<String, Object> data = tuple('k', '\u212A');
        System.assert(false, 'CAPTURE|asciiToKelvin|' + payload(data));
    }

    @IsTest static void asciiPositive() {
        Map<String, Object> data = tuple('K', 'k');
        System.assert(false, 'CAPTURE|kelvinAsciiPositive|' + payload(data));
    }

    @IsTest static void unrelatedNegative() {
        Map<String, Object> data = tuple('\u212A', 'z');
        System.assert(false, 'CAPTURE|kelvinNegative|' + payload(data));
    }
}
