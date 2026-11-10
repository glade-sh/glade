public class FernCaseLog { public static List<String> events=new List<String>(); public static String take(){ String s=String.join(events,',');events.clear();return s; } }
