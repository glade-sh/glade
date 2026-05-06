public class RunnerSchedule {
  public void execute(SchedulableContext context) {
    insert new Widget__c(Name = 'Scheduled');
  }
}
