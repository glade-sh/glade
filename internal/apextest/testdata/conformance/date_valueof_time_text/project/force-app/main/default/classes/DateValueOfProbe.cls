public class DateValueOfProbe {
    static String date(String text) {
        try { return String.valueOf(Date.valueOf(text)); }
        catch (Exception issue) { return issue.getTypeName() + ':' + issue.getMessage(); }
    }
    static String datetime(String text) {
        try { return String.valueOf(Datetime.valueOf(text)); }
        catch (Exception issue) { return issue.getTypeName() + ':' + issue.getMessage(); }
    }
    static String datetimeGmt(String text) {
        try { return Datetime.valueOfGmt(text).formatGmt('yyyy-MM-dd HH:mm:ss'); }
        catch (Exception issue) { return issue.getTypeName() + ':' + issue.getMessage(); }
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        List<String> samples = new List<String>{
            '2020-04-17', '2020-04-17 00:00', '2020-04-17 00:00:00', '2020-04-17 23:59:59', '2020-04-17T00:00:00', '2020-04-17T00:00',
            '2020-04-17 00:00:00.000', '2020-04-17 0:00', '2020-04-17 24:00:00', '2020-04-17 12', '2020-04-17 ', ' 2020-04-17',
            '2020-4-7', '2020-04-17 abc', '2020-04-17 00:00:00Z', '2020-04-17 10:20:30.123', '2020-04-17 25:00', '2020-04-17 10:20:30 +01:00',
            '2020-04-17 10:61'
        };
        Integer index = 0;
        for (String sample : samples) {
            result.put('s' + index, new Map<String,Object>{ 'in' => sample, 'date' => date(sample), 'datetime' => datetime(sample), 'gmt' => datetimeGmt(sample) });
            index++;
        }
        return result;
    }
}
