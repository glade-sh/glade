public class TraceProbe62 {
    public static String encoded(String value) {
        return value == null ? 'N' : 'B:'+EncodingUtil.base64Encode(Blob.valueOf(value));
    }
    public static String capture(String name) {
        try {
            TraceLeaf62.fail(name);
            return 'test:'+Test.isRunningTest()+'|case:'+name+'|no-exception';
        } catch (Exception e) {
            Exception cause = e.getCause();
            String causeType = cause == null ? 'N' : cause.getTypeName();
            String causeMessage = cause == null ? 'N' : encoded(cause.getMessage());
            return 'test:'+Test.isRunningTest()+'|case:'+name+'|type:'+e.getTypeName()+'|message:'+encoded(e.getMessage())+'|causeType:'+causeType+'|causeMessage:'+causeMessage+'|line:'+e.getLineNumber()+'|stack:'+encoded(e.getStackTraceString());
        }
    }
}
