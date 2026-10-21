Account row = new Account();
Boolean ready = true;
for (String fieldName : new List<String>{'FirstName', 'LastName'}) {
    try {
        row.put(fieldName, fieldName == 'FirstName' ? 'Ari' : 'Vale');
        System.debug('DYNAMIC_DML|PUT|' + fieldName + '|OK');
    } catch (Exception ex) {
        ready = false;
        System.debug('DYNAMIC_DML|PUT|' + fieldName + '|' + ex.getTypeName() + '|' + ex.getMessage());
    }
}
if (ready) {
    try {
        insert row;
        System.debug('DYNAMIC_DML|INSERT|OK');
    } catch (Exception ex) {
        System.debug('DYNAMIC_DML|INSERT|' + ex.getTypeName() + '|' + ex.getMessage());
    }
}
