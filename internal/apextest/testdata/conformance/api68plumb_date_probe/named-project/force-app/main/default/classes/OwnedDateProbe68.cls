public class OwnedDateProbe68 {
 public static String rejected() { try { JSON.deserialize('null', Date.class); return 'returned'; } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); } }
 public static String accepted() { return String.valueOf((Date)JSON.deserialize('"2026-11-03"', Date.class)); }
}
