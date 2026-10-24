@IsTest public class CdcMutationObservationTest {
    @IsTest static void selected_direct() { CdcGapObservation.emit('selected_direct'); }
    @IsTest static void cascade_delete() { CdcGapObservation.emit('cascade_delete'); }
    @IsTest static void merge_accounts() { CdcGapObservation.emit('merge_accounts'); }
    @IsTest static void external_upsert() { CdcGapObservation.emit('external_upsert'); }
}
