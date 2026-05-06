public class RunnerFuture {
  @future public static void mark(Integer amount) {
    insert new Widget__c(Name = 'Future-' + amount);
  }
}
