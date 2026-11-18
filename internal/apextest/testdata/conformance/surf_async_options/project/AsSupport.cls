public class AsSupport {
 public static List<String> hits = new List<String>();
 public static String noResult;
 public static String yesResult;
 public static Integer callBefore;
 public static Integer callAfter;
 public static List<String> depthSeen = new List<String>();
 public static List<String> dedupSeen = new List<String>();

 public class Job implements Queueable {
  public String tag = 'job';
  public void execute(QueueableContext ctx) {
   hits.add(tag);
  }
 }
 public class Chain implements Queueable {
  public Integer hop;
  public Integer limitHop;
  public Chain(Integer h, Integer lim) { hop = h; limitHop = lim; }
  public void execute(QueueableContext ctx) {
   hits.add('hop' + hop);
   if (hop < limitHop) {
    Chain next = new Chain(hop + 1, limitHop);
    try {
     System.enqueueJob(next);
    } catch (Exception e) { hits.add('stop' + hop + '=' + e.getTypeName() + '|' + e.getMessage()); }
   }
  }
 }
 public class NoCalloutJob implements Queueable {
  public void execute(QueueableContext ctx) {
   HttpRequest req = new HttpRequest();
   req.setEndpoint('https://example.invalid/probe');
   req.setMethod('GET');
   try { HttpResponse res = new Http().send(req); AsSupport.noResult = 'ok:' + res.getStatusCode(); }
   catch (Exception e) { AsSupport.noResult = e.getTypeName() + '|' + e.getMessage(); }
  }
 }
 public class CalloutJob implements Queueable, Database.AllowsCallouts {
  public void execute(QueueableContext ctx) {
   HttpRequest req = new HttpRequest();
   req.setEndpoint('https://example.invalid/probe');
   req.setMethod('GET');
   AsSupport.callBefore = Limits.getCallouts();
   try {
    HttpResponse res = new Http().send(req);
    AsSupport.callAfter = Limits.getCallouts();
    AsSupport.yesResult = 'ok:' + res.getStatusCode() + '|' + AsSupport.callBefore + '>' + AsSupport.callAfter;
   } catch (Exception e) { AsSupport.yesResult = e.getTypeName() + '|' + e.getMessage(); }
  }
 }
 public class Mock implements HttpCalloutMock {
  public HTTPResponse respond(HTTPRequest req) {
   HttpResponse res = new HttpResponse();
   res.setStatusCode(201);
   res.setBody('{"ok":true}');
   return res;
  }
 }
 public class DepthJob implements Queueable {
  public void execute(QueueableContext ctx) {
   try { AsSupport.depthSeen.add('cur=' + AsyncInfo.getCurrentQueueableStackDepth()); } catch (Exception e) { AsSupport.depthSeen.add('cur=' + e.getTypeName() + '|' + e.getMessage()); }
   try { AsSupport.depthSeen.add('max=' + AsyncInfo.getMaximumQueueableStackDepth()); } catch (Exception e) { AsSupport.depthSeen.add('max=' + e.getTypeName() + '|' + e.getMessage()); }
   try { AsSupport.depthSeen.add('has=' + AsyncInfo.hasMaxStackDepth()); } catch (Exception e) { AsSupport.depthSeen.add('has=' + e.getTypeName() + '|' + e.getMessage()); }
   try { AsSupport.depthSeen.add('delay=' + AsyncInfo.getMinimumQueueableDelayInMinutes()); } catch (Exception e) { AsSupport.depthSeen.add('delay=' + e.getTypeName() + '|' + e.getMessage()); }
  }
 }
 public class DedupChild implements Queueable {
  public void execute(QueueableContext ctx) {
   AsyncOptions a = new AsyncOptions();
   a.DuplicateSignature = QueueableDuplicateSignature.builder().addString('X').build();
   try { Id one = System.enqueueJob(new Job(), a); AsSupport.dedupSeen.add('one=ok'); } catch (Exception e) { AsSupport.dedupSeen.add('one=' + e.getTypeName() + '|' + e.getMessage()); }
   try { Id two = System.enqueueJob(new Job(), a); AsSupport.dedupSeen.add('two=ok'); } catch (Exception e) { AsSupport.dedupSeen.add('two=' + e.getTypeName() + '|' + e.getMessage()); }
  }
 }
}
