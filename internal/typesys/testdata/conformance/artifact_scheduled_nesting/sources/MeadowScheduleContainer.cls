public class MeadowScheduleContainer {
    public class InnerJob implements Schedulable {
        public void execute(SchedulableContext context) {
            insert new Account(Name = 'Meadow scheduled effect');
        }
    }
}
