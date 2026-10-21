public class QueueDataBoundaryProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        String suffix = String.valueOf(DateTime.now().getTime());
        insertObserved('account', new Account(Name='Boundary account'), observed);
        Group queue = new Group(Name='Boundary queue', DeveloperName='BoundaryQueue' + suffix, Type='Queue');
        insertObserved('group', queue, observed);
        insertObserved('queue_object', new QueueSobject(QueueId=queue.Id, SobjectType='Case'), observed);
        insertObserved('member', new GroupMember(GroupId=queue.Id, UserOrGroupId=UserInfo.getUserId()), observed);
        return observed;
    }

    private static void insertObserved(String stage, SObject row, Map<String,Object> observed) {
        try {
            insert row;
            observed.put(stage, new Map<String,Object>{'ok'=>true});
        } catch (DmlException issue) {
            observed.put(stage, new Map<String,Object>{
                'ok'=>false,
                'type'=>issue.getTypeName(),
                'status'=>issue.getNumDml() > 0 ? String.valueOf(issue.getDmlStatusCode(0)) : null
            });
        } catch (Exception issue) {
            observed.put(stage, new Map<String,Object>{'ok'=>false, 'type'=>issue.getTypeName()});
        }
    }
}
