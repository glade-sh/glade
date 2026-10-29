public class CpuProbe67 {
 public static void observe() {
  Integer before = Limits.getCpuTime();
  Integer sum = 0;
  for (Integer i=0;i<100000;i++) { sum += Math.mod(i, 97); }
  Integer after = Limits.getCpuTime();
  System.debug('CPU_ROW|work|' + before + '|' + after + '|' + Limits.getLimitCpuTime());
  HttpRequest req = new HttpRequest(); req.setEndpoint('https://httpbin.org/delay/2'); req.setMethod('GET'); req.setTimeout(20000);
  Long wall = DateTime.now().getTime(); Integer cpu = Limits.getCpuTime();
  HttpResponse response = new Http().send(req);
  Long waited = DateTime.now().getTime()-wall; Integer waitedCpu = Limits.getCpuTime()-cpu;
  System.assert(false, 'CPU_CAPTURE|' + before + '|' + after + '|' + Limits.getLimitCpuTime() + '|' + waited + '|' + waitedCpu + '|' + response.getStatusCode());
 }
 public static void exhaust() {
  try { while (true) { Integer used = Limits.getCpuTime(); } }
  catch (Exception e) { System.debug('CPU_CAUGHT|' + e.getTypeName()); }
 }
 public class Child implements Queueable {
  public void execute(QueueableContext c) { System.assertEquals(60000, Limits.getLimitCpuTime()); System.assert(Limits.getCpuTime() >= 0); }
 }
}
