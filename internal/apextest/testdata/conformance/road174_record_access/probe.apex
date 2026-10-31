for (String objectName : new List<String>{'Lead','Task','Event','Campaign','Contract','Asset','Product2','Pricebook2','Order','OpportunityLineItem','User','Profile','Account','Contact','Case'}) {
    try {
        Database.query('SELECT UserRecordAccess.HasReadAccess FROM ' + objectName + ' LIMIT 1');
        System.debug('ROAD174|' + objectName + '|ok');
    } catch (Exception e) {
        System.debug('ROAD174|' + objectName + '|reject|' + e.getMessage());
    }
}
