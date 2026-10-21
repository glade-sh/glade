for (String fieldName : new List<String>{'PersonEmail', 'IsPersonAccount'}) {
    try {
        List<SObject> rows = Database.query('SELECT ' + fieldName + ' FROM Account LIMIT 1');
        System.debug('DYNAMIC_SOQL|' + fieldName + '|OK|' + rows.size());
    } catch (Exception ex) {
        System.debug('DYNAMIC_SOQL|' + fieldName + '|' + ex.getTypeName() + '|' + ex.getMessage());
    }
}
