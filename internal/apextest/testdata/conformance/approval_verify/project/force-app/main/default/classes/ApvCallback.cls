public class ApvCallback implements Metadata.DeployCallback {
    public static Boolean ran = false;
    public static String status = 'unset';
    public void handleResult(Metadata.DeployResult result, Metadata.DeployCallbackContext context) {
        ran = true;
        status = String.valueOf(result.status);
    }
}
