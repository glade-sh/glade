// Local construction candidates only; native answers remain pending.
@IsTest private class LocatorCriteriaFormsCandidateTest {
    private static Database.QueryLocator construct(Set<Id> parents,String criterion) {
        return Database.getQueryLocator('SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND '+criterion+' GROUP BY AccountId');
    }
    private static void check(String criterion) {
        Database.QueryLocator handle=construct(new Set<Id>(),criterion);
        Assert.isNotNull(handle);
        Boolean caught=false;
        try { handle.iterator(); }
        catch(QueryException issue) { caught=true; Assert.areEqual('Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch',issue.getMessage()); }
        Assert.isTrue(caught);
    }
    private static void rejected(Set<Id> parents,String criterion) {
        Boolean caught=false;
        try { construct(parents,criterion); }
        catch(QueryException issue) { caught=true; Assert.areEqual('Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch',issue.getMessage()); }
        Assert.isTrue(caught);
    }
    @IsTest static void dateBeforeTodayCandidate(){check('CloseDate < TODAY');}
    @IsTest static void dateOrAmountCandidate(){check('(CloseDate < TODAY OR Amount > 100)');}
    @IsTest static void picklistNotEqualCandidate(){check('StageName != \'Prospecting\'');}
    @IsTest static void amountLessThanCandidate(){check('Amount < 200');}
    @IsTest static void picklistEqualCandidate(){check('StageName = \'Prospecting\'');}
    @IsTest static void nonemptyControl(){rejected(new Set<Id>{'001000000000099'},'Amount < 200');}
    @IsTest static void numericOperatorControl(){rejected(new Set<Id>(),'Amount >= 200');}
    @IsTest static void malformedLiteralControl(){rejected(new Set<Id>(),'Amount < \'invalid\'');}
    @IsTest static void missingFieldControl(){rejected(new Set<Id>(),'MissingDate < TODAY');}
    @IsTest static void malformedSyntaxControl(){Boolean caught=false;try{construct(new Set<Id>(),'Amount <');}catch(QueryException issue){caught=true;}Assert.isTrue(caught);}
}
