@IsTest
private class MeadowArtifactScheduleTest {
    @IsTest static void innerArtifact() {
        Test.startTest();
        System.schedule('Meadow inner schedule', '0 0 0 1 1 ? 2050', new MeadowScheduleContainer.InnerJob());
        Test.stopTest();
        Integer effects = [SELECT COUNT() FROM Account WHERE Name = 'Meadow scheduled effect'];
        Integer cron = [SELECT COUNT() FROM CronTrigger WHERE CronJobDetail.Name = 'Meadow inner schedule'];
        System.assert(false, 'OBS effects=' + effects + ' cron=' + cron);
    }
    @IsTest static void topArtifact() {
        Test.startTest();
        System.schedule('Meadow top schedule', '0 0 0 1 1 ? 2050', new MeadowTopSchedule());
        Test.stopTest();
        Integer effects = [SELECT COUNT() FROM Account WHERE Name = 'Meadow scheduled effect'];
        Integer cron = [SELECT COUNT() FROM CronTrigger WHERE CronJobDetail.Name = 'Meadow top schedule'];
        System.assert(false, 'OBS effects=' + effects + ' cron=' + cron);
    }
}
