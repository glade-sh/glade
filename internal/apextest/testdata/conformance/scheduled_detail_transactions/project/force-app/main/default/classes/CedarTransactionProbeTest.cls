@IsTest private class CedarTransactionProbeTest {
    @IsTest static void scheduleSameTransaction() { observe(false); }
    @IsTest static void scheduleBatchSameTransaction() { observe(true); }
    private static void observe(Boolean batch) {
        CedarTransactionProbe.submit(batch, 'cedar first transaction');
        System.assert(false, 'OBS|' + JSON.serialize(CedarTransactionProbe.observe(batch, 'cedar first transaction', 'cedar second transaction')));
    }
}
