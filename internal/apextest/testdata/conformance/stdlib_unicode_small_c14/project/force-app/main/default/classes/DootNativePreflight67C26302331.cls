@IsTest
private class DootNativePreflight67C26302331 {
    @IsTest static void baseline() {
        System.assertEquals(2, 1 + 1, 'BASELINE|arithmetic');
    }

    private static Map<String, Object> span(Integer y1, Integer m1, Integer d1,
                                             Integer y2, Integer m2, Integer d2) {
        Map<String, Object> row = new Map<String, Object>();
        String stage = 'dateConstruction';
        try {
            Date startDate = Date.newInstance(y1, m1, d1);
            Date endDate = Date.newInstance(y2, m2, d2);
            stage = 'daysBetween';
            Integer delta = startDate.daysBetween(endDate);
            row.put('status', 'ok');
            row.put('value', delta);
        } catch (Exception e) {
            row.put('status', 'exception');
            row.put('stage', stage);
            row.put('exceptionType', e.getTypeName());
            row.put('exceptionMessage', e.getMessage());
        }
        return row;
    }

    @IsTest static void date400() {
        Map<String, Object> data = new Map<String, Object>{
            'sourceApi' => '67.0',
            'route' => 'named-test-check-only',
            'locale' => UserInfo.getLocale(),
            'language' => UserInfo.getLanguage(),
            'timeZone' => UserInfo.getTimeZone().getID(),
            'forward1800To2200' => span(1800, 1, 1, 2200, 1, 1),
            'reverse2200To1800' => span(2200, 1, 1, 1800, 1, 1),
            'zero' => span(2000, 1, 1, 2000, 1, 1),
            'adjacentForward' => span(2000, 1, 1, 2000, 1, 2),
            'adjacentReverse' => span(2000, 1, 2, 2000, 1, 1)
        };
        // Intentional observation transport, not an assertion of a native answer.
        System.assert(false, 'CAPTURE|date400|' + JSON.serialize(data));
    }
}
