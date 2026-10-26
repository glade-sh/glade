public class RouteTwinProbe {
    public static void submit() {
        String cron=Datetime.now().addSeconds(25).format('s m H d M ? yyyy');
        String plain=System.enqueueJob(new RouteIdentityQueue('plain'));
        String delayed=System.enqueueJob(new RouteIdentityQueue('integer'),0);
        String options=System.enqueueJob(new RouteIdentityQueue('options'),new AsyncOptions());
        insert new List<Account>{new Account(Name='plainReturn',Phone=plain),new Account(Name='integerReturn',Phone=delayed),new Account(Name='optionsReturn',Phone=options)};
        System.enqueueJob(new InstanceQueueBase()); System.enqueueJob(new InstanceQueueChild());
        System.enqueueJob(new StaticQueueBase());
        System.schedule('instance declared '+Datetime.now().getTime(),cron,new InstanceScheduleBase());
        System.schedule('instance inherited '+Datetime.now().getTime(),cron,new InstanceScheduleChild());
        System.schedule('static declared '+Datetime.now().getTime(),cron,new StaticScheduleBase());
        Database.executeBatch(new InstanceBatchBase()); Database.executeBatch(new InstanceBatchChild());
        Database.executeBatch(new StaticBatchBase());
    }
    public static void submitScheduledBatch() {
        System.scheduleBatch(new InstanceBatchBase(),'instance declared batch '+Datetime.now().getTime(),1);
        System.scheduleBatch(new InstanceBatchChild(),'instance inherited batch '+Datetime.now().getTime(),1);
        System.scheduleBatch(new StaticBatchBase(),'static declared batch '+Datetime.now().getTime(),1);
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result=new Map<String,Object>();
        Map<String,String> returns=new Map<String,String>();
        for(Account a:[SELECT Name,Phone FROM Account WHERE Name LIKE '%Return']) returns.put(a.Name,a.Phone);
        for(Account a:[SELECT Name,Phone,Description FROM Account WHERE Name IN ('plain','integer','options','plainReturn','integerReturn','optionsReturn','instanceQueue','instanceSchedule','instanceBatchStart','instanceBatchExecute','instanceBatchFinish','staticQueue','staticSchedule','staticBatchStart','staticBatchExecute','staticBatchFinish') ORDER BY Name]) {
            if(a.Name.endsWith('Return')) continue;
            if(a.Description!=null) result.put(a.Name,new List<Object>{JSON.deserializeUntyped(a.Description),a.Phone.equals(returns.get(a.Name+'Return')),returns.get(a.Name+'Return').length()});
            else { Integer count=result.containsKey(a.Name)?(Integer)result.get(a.Name):0; result.put(a.Name,count+1); }
        }
        return result;
    }
}
