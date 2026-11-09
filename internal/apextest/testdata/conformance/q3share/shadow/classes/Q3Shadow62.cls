public class Q3Shadow62 {
 public class Reason { public String Manual = 'local'; }
 public class Share { public Reason RowCause = new Reason(); }
 public class Root { public Share AccountShare = new Share(); }
 public static void run() {
  Share AccountShare = new Share(); System.assertEquals('local', AccountShare.RowCause.Manual);
  Root Schema = new Root(); System.assertEquals('local', Schema.AccountShare.RowCause.Manual);
 }
}