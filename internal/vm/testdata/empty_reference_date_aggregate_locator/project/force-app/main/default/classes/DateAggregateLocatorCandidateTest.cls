// Native-observed empty Date aggregate construction and iteration, with local
// scope controls retaining unobserved aggregate restrictions.
@IsTest private class DateAggregateLocatorCandidateTest {
    private static Database.QueryLocator construct(Set<Id> parents,String aggregate,String criterion) {
        parents=parents.clone();
        return Database.getQueryLocator('SELECT AccountId,'+aggregate+' resultValue FROM Opportunity WHERE AccountId IN :parents AND ('+criterion+') GROUP BY AccountId');
    }
    private static void check(String aggregate,String criterion) {
        Database.QueryLocator handle=construct(new Set<Id>(),aggregate,criterion);
        Assert.isNotNull(handle);
        Database.QueryLocatorIterator rows=handle.iterator();
        Assert.isNotNull(rows);
        Assert.isFalse(rows.hasNext());
    }
    private static void rejected(Set<Id> parents,String aggregate,String criterion) {
        Boolean caught=false;
        try { construct(parents,aggregate,criterion); }
        catch(QueryException issue) { caught=true; }
        Assert.isTrue(caught);
    }
    @IsTest static void maximumDateCandidate(){check('MAX(CloseDate)','CloseDate < TODAY');}
    @IsTest static void maximumDateOrDecimalCandidate(){check('MAX(CloseDate)','CloseDate < TODAY OR Amount > 100');}
    @IsTest static void countDateCandidate(){check('COUNT(CloseDate)','Amount < 200');}
    @IsTest static void nonemptyControls(){for(String aggregate:new List<String>{'MAX(CloseDate)','COUNT(CloseDate)'}){rejected(new Set<Id>{'001000000000099'},aggregate,aggregate.startsWith('MAX')?'CloseDate < TODAY':'Amount < 200');}}
    @IsTest static void otherAggregateControls(){rejected(new Set<Id>(),'MIN(CloseDate)','CloseDate < TODAY');rejected(new Set<Id>(),'COUNT(Id)','Amount < 200');}
    @IsTest static void malformedControls(){rejected(new Set<Id>(),'MAX(CloseDate)','MissingDate < TODAY');rejected(new Set<Id>(),'COUNT(CloseDate)','Amount <');}
}
