OwnedSafeNavigationBindHolder missing = null;
try {
    Id selected = [SELECT Id FROM Account WHERE Name = :missing?.Name]?.Id;
    System.assertEquals(null, selected);
    System.debug(LoggingLevel.ERROR, 'OBS=value:' + String.valueOf(selected));
} catch (Exception e) {
    System.debug(LoggingLevel.ERROR, 'OBS=exception:' + e.getTypeName() + ':' + e.getMessage());
}
