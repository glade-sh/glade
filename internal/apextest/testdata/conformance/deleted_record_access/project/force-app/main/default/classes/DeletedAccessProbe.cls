public class DeletedAccessProbe {
    private static Map<String,Object> accessFor(Id recordId) {
        Id actorId = UserInfo.getUserId();
        List<UserRecordAccess> rows = [
            SELECT RecordId, HasReadAccess, HasEditAccess, HasDeleteAccess,
                HasTransferAccess, HasAllAccess, MaxAccessLevel
            FROM UserRecordAccess
            WHERE RecordId = :recordId AND UserId = :actorId
        ];
        Map<String,Object> result = new Map<String,Object>{'size'=>rows.size()};
        if (!rows.isEmpty()) {
            result.put('read', rows[0].HasReadAccess);
            result.put('edit', rows[0].HasEditAccess);
            result.put('delete', rows[0].HasDeleteAccess);
            result.put('transfer', rows[0].HasTransferAccess);
            result.put('all', rows[0].HasAllAccess);
            result.put('level', rows[0].MaxAccessLevel);
            result.put('sameRecord', rows[0].RecordId == recordId);
        }
        return result;
    }

    public static Map<String,Object> observe() {
        Account target = new Account(Name='Owned restored access');
        Account control = new Account(Name='Owned live access');
        insert new List<Account>{target, control};
        Map<String,Object> observed = new Map<String,Object>();
        observed.put('before', accessFor(target.Id));
        observed.put('liveBefore', accessFor(control.Id));
        delete target;
        observed.put('deleted', accessFor(target.Id));
        observed.put('liveAfterDelete', accessFor(control.Id));
        observed.put('visibleDeleted', [SELECT COUNT() FROM Account WHERE Id = :target.Id]);
        undelete target;
        observed.put('restored', accessFor(target.Id));
        observed.put('liveAfterRestore', accessFor(control.Id));
        observed.put('visibleRestored', [SELECT COUNT() FROM Account WHERE Id = :target.Id]);
        return observed;
    }
}
