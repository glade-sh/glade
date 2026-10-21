public interface AmountFormatter {
 String format(Decimal amount);
 String format(Decimal amount, String pattern);
}
