public without sharing class CapFlowCaller67 {
    public static void run(String flowApiName, Id accountId) {
        try {
            Flow.Interview iv = Flow.Interview.createInterview(flowApiName,
                new Map<String,Object>{'recordId' => accountId});
            iv.start();
            System.debug(LoggingLevel.ERROR, 'FLOW=SUCCESS;FLOW_NAME=' + flowApiName);
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'FLOW_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
    public static void runAsAdmin(String flowApiName, Id accountId) {
        run(flowApiName, accountId);
    }
}
