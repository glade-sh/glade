@IsTest private class CacheStaticEntryTest {
    @IsTest static void organizationPartition(){
        Cache.OrgPartition partition=CacheAccess.organization();
        Assert.areEqual('local.default',partition.getName());
        partition.put('entry','stored');Assert.areEqual('stored',partition.get('entry'));
        Assert.isTrue(partition.contains('entry'));Assert.isTrue(partition.getKeys().contains('entry'));
        Assert.isTrue(partition.remove('entry'));Assert.isFalse(partition.contains('entry'));
    }
    @IsTest static void sessionAvailability(){Assert.isFalse(CacheAccess.available());}
    @IsTest static void sessionPartition(){
        Cache.SessionPartition partition=CacheAccess.session();
        partition.put('entry','stored');Assert.areEqual('stored',partition.get('entry'));
        Assert.isTrue(partition.remove('entry'));
    }
    @IsTest static void partitionQualificationControl(){
        Assert.areEqual('local.Main.entry',Cache.OrgPartition.createFullyQualifiedKey('local','Main','entry'));
        Assert.areEqual('local.Main',Cache.SessionPartition.createFullyQualifiedPartition('local','Main'));
    }
}
