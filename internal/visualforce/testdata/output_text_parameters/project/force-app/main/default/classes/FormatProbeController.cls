public class FormatProbeController {
 public List<Decimal> amounts { get { return new List<Decimal>{12345.625}; } }
 public Decimal amount { get { return 12345.625; } }
 public Decimal negative { get { return -12345.625; } }
 public Decimal tie { get { return 2.5; } }
 public Decimal zero { get { return 0; } }
 public String caption { get { return '<em>Blue & green</em>'; } }
 public String missing { get { return null; } }
 public Date day { get { return Date.newInstance(2024, 2, 9); } }
 public Datetime stamp { get { return Datetime.newInstanceGmt(2024, 2, 9, 17, 6, 3); } }
}
