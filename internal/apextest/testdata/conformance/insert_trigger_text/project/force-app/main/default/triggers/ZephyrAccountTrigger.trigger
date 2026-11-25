trigger ZephyrAccountTrigger on Account (before insert) {
    for (Account row : Trigger.new) {
        ZephyrTextProbe.observed.put('account-before-name', row.Name);
        ZephyrTextProbe.observed.put('account-before-site', row.Site);
    }
}
