@IsTest private class GovernorEdgeProbeTest {
    @IsTest static void C300() { GovernorEdgeProbe.observe('children', 75); System.assert(false, 'EDGE|' + Limits.getQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows()); }
    @IsTest static void C304() { GovernorEdgeProbe.observe('children', 76); System.assert(false, 'EDGE|' + Limits.getQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows()); }
    @IsTest static void Q100() { GovernorEdgeProbe.observe('queries', 100); System.assert(false, 'EDGE|' + Limits.getQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows()); }
    @IsTest static void Q101() { GovernorEdgeProbe.observe('queries', 101); System.assert(false, 'EDGE|' + Limits.getQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows()); }
    @IsTest static void D150() { GovernorEdgeProbe.observe('dml', 150); System.assert(false, 'EDGE|' + Limits.getQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows()); }
    @IsTest static void D151() { GovernorEdgeProbe.observe('dml', 151); System.assert(false, 'EDGE|' + Limits.getQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows()); }
    @IsTest static void D000() { GovernorEdgeProbe.observe('emptyDml', 151); System.assert(false, 'EDGE|' + Limits.getQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows()); }
}
