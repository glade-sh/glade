public class AmountFacade {
 public static String invoke(AmountFormatter service) {
  return service.format(7) + '|' + service.format(7, 'pattern');
 }
}
