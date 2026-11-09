public virtual class NsHolder { public class Quote { public Integer v = 1; public Quote() {} } public enum Survey { A, B } public interface Territory2 { Integer n(); } public Quote q = new Quote(); public List<Quote> ql = new List<Quote>(); public Survey s = Survey.A;
 public static String inside() { Quote z = new Quote(); Survey y = Survey.B; List<Quote> l = new List<Quote>{z}; Object o = z; Boolean b = o instanceof Quote; Quote c = (Quote) o; Type t = Quote.class; return z.v + '|' + y + '|' + l.size() + '|' + b + '|' + c.v + '|' + t; }
 public class Sib { public Quote make() { return new Quote(); } }
 public static Integer viaSib() { return new Sib().make().v; }
 public static Integer fe() { Integer n = 0; for (Quote x : new List<Quote>{new Quote(), new Quote()}) n += x.v; return n; }
 public static String impl() { Territory2 t = new Impl(); return '' + t.n(); }
 public class Impl implements Territory2 { public Integer n() { return 7; } } }
