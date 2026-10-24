@IsTest private class CedarAuditProbeTest {
    @TestSetup static void seed() { insert new Account(Name='Cedar setup'); }
    @IsTest static void observeRows() { System.assert(false,'PROBE|'+JSON.serialize(CedarAuditProbe.observe(true))); }
}
