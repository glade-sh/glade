public class CrossValue62 {
 public static Integer visits=0;
 public static String field { get { visits++; if (visits==1) CrossHelper62.read(); return 'ok'; } set; }
 public static String readOnly { get { visits++; if (visits==1) CrossHelper62.readOnly(); return 'ok'; } }
 public static String direct { get { visits++; if (visits==1) { String currentValue=direct; } return 'ok'; } set; }
 public static String recursiveValue { get { return CrossHelper62.cycle(); } set; }
}
