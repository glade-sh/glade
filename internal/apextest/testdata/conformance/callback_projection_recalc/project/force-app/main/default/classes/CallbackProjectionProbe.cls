public class CallbackProjectionProbe {
    public virtual class ParentProcessor {
        public String label;
    }

    public class SpecificProcessor extends ParentProcessor {
    }

    public class DispatchSink {
        public String selected;

        public void record(ParentProcessor value) {
            selected = 'parent';
        }

        public void record(SpecificProcessor value) {
            selected = 'specific';
        }
    }

    public class DispatchOwner {
        public SpecificProcessor processor = new SpecificProcessor();

        public void dispatch(DispatchSink sink) {
            sink.record(processor);
        }
    }

    public class RateWindow {
        public Decimal rate;
        public Date startDate;
        public Date nextStartDate;
    }

    public static Map<String, Object> observe() {
        Savepoint point = Database.setSavepoint();
        Map<String, Object> result = new Map<String, Object>();
        try {
            result.put('callbackStaticOverloadSelection', observeCallback());
            result.put('parentProjectionAfterIdQuery', observeParentProjection());
            result.put('datedCurrencySelection', observeDatedCurrency());
            result.put('cachedAverageAfterChildMutation', observeCachedAverage());
            result.put('cachedFirstLastAfterChildMutation', observeCachedFirstLast());
            Database.rollback(point);
            return result;
        } catch (Exception e) {
            Database.rollback(point);
            throw e;
        }
    }

    private static String observeCallback() {
        DispatchSink sink = new DispatchSink();
        new DispatchOwner().dispatch(sink);
        return sink.selected;
    }

    private static Boolean observeParentProjection() {
        Account parent = new Account(Name = 'Probe Parent');
        insert parent;
        Account idOnly = [SELECT Id FROM Account WHERE Id = :parent.Id];
        Account refreshed = [SELECT Id, Name FROM Account WHERE Id = :idOnly.Id];
        Map<String, Object> fields = refreshed.getPopulatedFieldsAsMap();
        return fields.containsKey('Name') && refreshed.Name == 'Probe Parent';
    }

    private static Decimal observeDatedCurrency() {
        Opportunity row = new Opportunity(
            Name = 'Probe Dated Currency',
            StageName = 'Prospecting',
            CloseDate = Date.newInstance(2025, 6, 15)
        );
        insert row;
        Opportunity projected = [SELECT CloseDate FROM Opportunity WHERE Id = :row.Id];

        RateWindow basic = new RateWindow();
        basic.rate = 5.0;
        basic.startDate = Date.newInstance(2000, 1, 1);

        RateWindow dated = new RateWindow();
        dated.rate = 5.5;
        dated.startDate = Date.newInstance(2025, 1, 1);
        dated.nextStartDate = Date.newInstance(2026, 1, 1);

        Date configuredDate = projected.CloseDate;
        Decimal selected = basic.rate;
        if (configuredDate >= dated.startDate && configuredDate < dated.nextStartDate) {
            selected = dated.rate;
        }
        return selected;
    }

    private static Decimal observeCachedAverage() {
        Account parent = new Account(Name = 'Probe Average Parent');
        insert parent;
        List<Opportunity> children = new List<Opportunity>{
            opportunity('Probe Average A', parent.Id, Date.newInstance(2025, 1, 1), 2),
            opportunity('Probe Average B', parent.Id, Date.newInstance(2025, 2, 1), 6)
        };
        insert children;

        Decimal beforeValue = (Decimal) [
            SELECT AVG(Amount) averageValue FROM Opportunity WHERE AccountId = :parent.Id
        ][0].get('averageValue');
        children[0].Amount = 10;
        update children[0];
        Decimal afterValue = (Decimal) [
            SELECT AVG(Amount) averageValue FROM Opportunity WHERE AccountId = :parent.Id
        ][0].get('averageValue');
        return beforeValue * 10 + afterValue;
    }

    private static String observeCachedFirstLast() {
        Account parent = new Account(Name = 'Probe Ordered Parent');
        insert parent;
        List<Opportunity> children = new List<Opportunity>{
            opportunity('Probe Ordered A', parent.Id, Date.newInstance(2025, 1, 1), 2),
            opportunity('Probe Ordered B', parent.Id, Date.newInstance(2025, 2, 1), 6),
            opportunity('Probe Ordered C', parent.Id, Date.newInstance(2025, 3, 1), 10)
        };
        insert children;

        List<Opportunity> beforeRows = [
            SELECT Id, Name, CloseDate FROM Opportunity WHERE AccountId = :parent.Id ORDER BY CloseDate ASC
        ];
        children[0].CloseDate = Date.newInstance(2025, 4, 1);
        children[2].CloseDate = Date.newInstance(2025, 5, 1);
        update children;
        List<Opportunity> afterRows = [
            SELECT Id, Name, CloseDate FROM Opportunity WHERE AccountId = :parent.Id ORDER BY CloseDate ASC
        ];
        return beforeRows[0].Name + '/' + beforeRows[beforeRows.size() - 1].Name + '>' +
            afterRows[0].Name + '/' + afterRows[afterRows.size() - 1].Name;
    }

    private static Opportunity opportunity(String name, Id accountId, Date closeDate, Decimal amount) {
        return new Opportunity(
            Name = name,
            AccountId = accountId,
            StageName = 'Prospecting',
            CloseDate = closeDate,
            Amount = amount
        );
    }
}
