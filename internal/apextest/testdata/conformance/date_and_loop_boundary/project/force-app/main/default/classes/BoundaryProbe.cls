public class BoundaryProbe {
    public static Map<String,Object> loopBelow() {
        Integer count = 0;
        while (count < 200) { count++; }
        return new Map<String,Object>{'count'=>count};
    }
    public static Map<String,Object> loopAbove() {
        Integer count = 0;
        while (count < 1000001) { count++; }
        return new Map<String,Object>{'count'=>count};
    }
    public static Map<String,Object> timeAdvance() {
        Long started = System.currentTimeMillis();
        Integer reads = 0;
        while (System.currentTimeMillis() - started < 20 && reads < 1000001) {
            reads++;
        }
        return new Map<String,Object>{'advanced'=>System.currentTimeMillis()-started>=20,'bounded'=>reads<1000001};
    }
    public static Map<String,Object> dateLiteral() {
        Account value = new Account(Name='Owned literal date boundary');
        insert value;
        Test.setCreatedDate(value.Id, Date.newInstance(2020,1,2));
        return audit(value.Id);
    }
    public static Map<String,Object> dateVariable() {
        Account value = new Account(Name='Owned variable date boundary');
        insert value;
        Date supplied = Date.newInstance(2020,1,5).addDays(-3);
        Test.setCreatedDate(value.Id, supplied);
        return audit(value.Id);
    }
    public static Map<String,Object> datetimeControl() {
        Account value = new Account(Name='Owned datetime boundary');
        insert value;
        Test.setCreatedDate(value.Id, Datetime.newInstanceGmt(2020,1,2,3,4,5));
        return audit(value.Id);
    }
    private static Map<String,Object> audit(Id identity) {
        Datetime stored = [SELECT CreatedDate FROM Account WHERE Id=:identity].CreatedDate;
        return new Map<String,Object>{'gmt'=>stored.formatGmt('yyyy-MM-dd HH:mm:ss')};
    }
}
