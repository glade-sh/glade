public without sharing class CasePhaseCapture {
    public static Id readerId;
    public static Id submitterId;
    public static Id escalationQueueId;
    public static String afterInsertAccess = 'unset';
    public static String afterInsertShareRows = 'unset';
    public static String escalationDuringState = 'unset';
    public static String ownerKind(Id ownerId) {
        if (ownerId == null) return 'no-owner';
        if (ownerId == submitterId) return 'submitter';
        if (ownerId == escalationQueueId) return 'escalation-queue';
        return 'other';
    }
    public static void observe(List<Case> rows) {
        if (rows.isEmpty()) return;
        for (Case row : rows) {
            if (row.Subject == 'NATIVE_CASE_SHARE_PROBE' && readerId != null && afterInsertAccess == 'unset') {
                List<UserRecordAccess> accessRows = [SELECT RecordId, HasReadAccess FROM UserRecordAccess WHERE UserId = :readerId AND RecordId = :row.Id];
                afterInsertAccess = accessRows.isEmpty() ? 'no-row' : String.valueOf(accessRows[0].HasReadAccess);
                afterInsertShareRows = String.valueOf([SELECT COUNT() FROM CaseShare WHERE CaseId = :row.Id AND RowCause = 'Rule']);
            }
            if (row.Subject == 'NATIVE_CASE_ESCALATION_PROBE' && escalationDuringState == 'unset') {
                Case saved = [SELECT Id, OwnerId, IsEscalated FROM Case WHERE Id = :row.Id];
                escalationDuringState = String.valueOf(saved.IsEscalated) + ':' + ownerKind(saved.OwnerId);
            }
        }
    }
}
