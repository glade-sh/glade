public class CrossValue67 {
 public static Integer visits=0;
 public static String field { get { visits++; if (visits==1) CrossHelper67.read(); return 'ok'; } set; }
 public static String readOnly { get { visits++; if (visits==1) CrossHelper67.readOnly(); return 'ok'; } }
 public static String direct { get { visits++; if (visits==1) { String currentValue=direct; } return 'ok'; } set; }
 public static String recursiveValue { get { return CrossHelper67.cycle(); } set; }
}
