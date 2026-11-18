public class ResidualCallout {
 @InvocableMethod(label='Residual call')
 public static void run(List<Boolean> items) {
  HttpRequest r = new HttpRequest(); r.setEndpoint('https://example.test/x'); r.setMethod('GET');
  new Http().send(r);
 }
}
