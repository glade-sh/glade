public class AmountService implements AmountFormatter {
 public String format(Decimal amount) { return 'real'; }
 public String format(Decimal amount, String pattern) { return 'real'; }
 public String scalar(Long count, Double ratio) { return 'real'; }
 public Object erased(Object value) { return value; }
}
