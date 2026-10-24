public class ScalarTextProbe {
 public static Boolean startswith_match() { insert new Account(Name='ScalarText_startswith_match'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean startswith_mismatch() { insert new Account(Name='ScalarText_startswith_mismatch'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean startswith_case() { insert new Account(Name='ScalarText_startswith_case'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean startswith_null() { insert new Account(Name='ScalarText_startswith_null'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean startswith_empty() { insert new Account(Name='ScalarText_startswith_empty'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean endswith_match() { insert new Account(Name='ScalarText_endswith_match'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean endswith_mismatch() { insert new Account(Name='ScalarText_endswith_mismatch'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean endswith_case() { insert new Account(Name='ScalarText_endswith_case'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean endswith_null() { insert new Account(Name='ScalarText_endswith_null'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean endswith_empty() { insert new Account(Name='ScalarText_endswith_empty'); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean direct_startswith_null() { Flow.Interview interview=Flow.Interview.createInterview('ScalarText_direct_startswith_null',new Map<String,Object>()); interview.start(); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean direct_startswith_empty() { Flow.Interview interview=Flow.Interview.createInterview('ScalarText_direct_startswith_empty',new Map<String,Object>()); interview.start(); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean direct_endswith_null() { Flow.Interview interview=Flow.Interview.createInterview('ScalarText_direct_endswith_null',new Map<String,Object>()); interview.start(); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
 public static Boolean direct_endswith_empty() { Flow.Interview interview=Flow.Interview.createInterview('ScalarText_direct_endswith_empty',new Map<String,Object>()); interview.start(); return [SELECT count() FROM Task WHERE Subject='Scalar text match'] > 0; }
}
