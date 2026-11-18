public class Parcel67 {
 public interface Builder { Builder build(String v); String value(); }
 public static Builder Builder = new Worker('a');
 public static Builder build(String v){return new Worker(v);}
 private class Worker implements Builder {
  private String s; public Worker(String v){s=v;}
  public Builder build(String v){s+=v; return this;}
  public String value(){return s;}
 }
 @AuraEnabled public static String run() { return Parcel67.Builder.build('b').value(); } }
