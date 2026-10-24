public class CedarLibrary {
 public Integer amount;
 public CedarLibrary(Integer n) { amount = n; }
 public static Integer value() { return 7; }
 public static void apply(List<Account> rows) { for (Account row : rows) row.Description = 'cedar'; }
}
