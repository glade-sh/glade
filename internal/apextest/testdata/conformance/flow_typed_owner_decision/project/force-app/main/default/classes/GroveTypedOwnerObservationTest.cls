@IsTest
private class GroveTypedOwnerObservationTest {
    @IsTest
    static void observeOwner() {
        Group queue = new Group(
            Name = 'Grove typed owner queue',
            DeveloperName = 'GroveTypedOwnerQueue',
            Type = 'Queue'
        );
        insert queue;
        insert new QueueSobject(QueueId = queue.Id, SobjectType = 'Case');

        Map<String, String> observations = new Map<String, String>();
        System.runAs(new User(Id = UserInfo.getUserId())) {
            Case activeUserCase = new Case(
                Subject = 'Grove active user owner',
                Status = 'New',
                Origin = 'Web',
                OwnerId = UserInfo.getUserId()
            );
            Case queueCase = new Case(
                Subject = 'Grove queue owner',
                Status = 'New',
                Origin = 'Web',
                OwnerId = queue.Id
            );
            insert new List<Case>{activeUserCase, queueCase};
            observations.put('activeUser', [SELECT Description FROM Case WHERE Id = :activeUserCase.Id].Description);
            observations.put('queueOwner', [SELECT Description FROM Case WHERE Id = :queueCase.Id].Description);
        }
        System.assert(false, 'OBSERVED|typedOwner|' + JSON.serialize(observations));
    }
}
