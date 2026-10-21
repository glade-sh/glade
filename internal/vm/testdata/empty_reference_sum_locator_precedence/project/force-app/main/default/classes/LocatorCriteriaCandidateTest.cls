// Local precedence candidate; native aggregate-plus-missing-field order is pending.
@IsTest private class LocatorCriteriaCandidateTest {
    private static String queryText(String criterion) {
        return 'SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND ' + criterion + ' GROUP BY AccountId';
    }
    private static Database.QueryLocator construct(Set<Id> parents, String criterion) {
        return Database.getQueryLocator(queryText(criterion));
    }
    @IsTest static void missingCriterionPrecedence() {
        Boolean caught = false;
        try { construct(new Set<Id>(), 'MissingCriterion = \'Synthetic\''); }
        catch (QueryException issue) {
            caught = true;
            Assert.isTrue(issue.getMessage().startsWith('No such column \'MissingCriterion\' on entity \'Opportunity\'.'));
        }
        Assert.isTrue(caught);
    }
    @IsTest static void validNoneligibleControl() {
        Boolean caught = false;
        try { construct(new Set<Id>(), 'Amount >= 200'); }
        catch (QueryException issue) {
            caught = true;
            Assert.areEqual('Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch', issue.getMessage());
        }
        Assert.isTrue(caught);
    }
    @IsTest static void eligibleConstructionControl() {
        Database.QueryLocator handle = construct(new Set<Id>(), 'Amount > 200');
        Assert.isNotNull(handle);
        Boolean caught = false;
        try { handle.iterator(); }
        catch (QueryException issue) {
            caught = true;
            Assert.areEqual('Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch', issue.getMessage());
        }
        Assert.isTrue(caught);
    }
    @IsTest static void malformedLiteralControl() {
        Boolean caught = false;
        try { construct(new Set<Id>(), 'Amount > \'invalid\''); }
        catch (QueryException issue) {
            caught = true;
            Assert.areEqual('Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch', issue.getMessage());
        }
        Assert.isTrue(caught);
    }
    @IsTest static void malformedSyntaxControl() {
        Boolean caught = false;
        try { construct(new Set<Id>(), 'Amount >'); }
        catch (QueryException issue) { caught = true; }
        Assert.isTrue(caught);
    }
}
