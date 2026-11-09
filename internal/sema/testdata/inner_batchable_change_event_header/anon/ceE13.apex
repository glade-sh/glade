AccountChangeEvent ev = new AccountChangeEvent();
EventBus.ChangeEventHeader h = new EventBus.ChangeEventHeader();
h.changeType = 'UPDATE'; h.entityName = 'Account'; h.recordIds = new List<String>{'001000000000001AAA'};
h.commitUser = '005000000000001AAA'; h.changedFields = new List<String>{'Name'};
Object v = ev.ChangeEventHeader.entityName;
System.debug('OBS|' + JSON.serialize(v));
