@IsTest private class LsCeTE15 {
 @IsTest static void observe() {
AccountChangeEvent ev = new AccountChangeEvent();
EventBus.ChangeEventHeader h = new EventBus.ChangeEventHeader();
h.changeType = 'UPDATE'; h.entityName = 'Account'; h.recordIds = new List<String>{'001000000000001AAA'};
h.commitUser = '005000000000001AAA'; h.changedFields = new List<String>{'Name'};
ev.ChangeEventHeader = h;
Object v = ev.ChangeEventHeader.recordIds;
System.assert(false, 'OBS|' + JSON.serialize(v));
 }
}
