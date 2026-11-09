public without sharing class IspInventory62 {
    private static final String SOURCE_API='62.0';
    private static final Integer BOUND=2001;
    private static List<String> payload;
    private static Map<String,Object> submission;
    private static final List<String> FIELDS=new List<String>{'Id','Name','Label','IsOwnedByProfile','IsCustom','Type','LicenseId','PermissionSetGroupId','NamespacePrefix','PermissionsApiUserOnly','PermissionsModifyAllData','PermissionsViewAllData'};
    private static void row(Map<String,Object> value) {payload.add(JSON.serialize(value));}
    private static String enc(String value) {return EncodingUtil.base64Encode(Blob.valueOf(value));}
    public static void beginCapture(String route) {
        payload=new List<String>();
        submission=new Map<String,Object>{'organizationId'=>UserInfo.getOrganizationId(),'userId'=>UserInfo.getUserId(),'profileId'=>UserInfo.getProfileId(),'sourceApi'=>SOURCE_API,'route'=>route,'phase'=>'A'};
        Map<String,Object> context=new Map<String,Object>();context.putAll(submission);context.put('kind','context');context.put('isTest',Test.isRunningTest());context.put('locale',UserInfo.getLocale());context.put('timezone',UserInfo.getTimeZone().getID());context.put('language',UserInfo.getLanguage());row(context);
    }
    private static void queryInventory(String queryId, Boolean nonProfile) {
        String predicate=nonProfile?' WHERE IsOwnedByProfile = false':'';
        String exactQuery='SELECT '+String.join(FIELDS,', ')+' FROM PermissionSet'+predicate;
        Integer reportedCount=Database.countQuery('SELECT COUNT() FROM PermissionSet'+predicate);
        List<SObject> records=Database.query(exactQuery+' WITH SYSTEM_MODE ORDER BY Id LIMIT 2001');
        Boolean complete=records.size()<BOUND && records.size()==reportedCount;
        row(new Map<String,Object>{'kind'=>'queryHeader','queryId'=>queryId,'query'=>exactQuery,'executedQuery'=>exactQuery+' WITH SYSTEM_MODE ORDER BY Id LIMIT 2001','reportedCount'=>reportedCount,'complete'=>complete,'method'=>'bounded-unsaturated','bound'=>BOUND});
        for(Integer i=0;i<records.size();i++) {
            Map<String,Object> value=new Map<String,Object>();for(String field:FIELDS)value.put(field,records[i].get(field));
            row(new Map<String,Object>{'kind'=>'queryRow','queryId'=>queryId,'index'=>i,'record'=>value});
        }
        row(new Map<String,Object>{'kind'=>'queryFooter','queryId'=>queryId,'rowCount'=>records.size()});
        if(!complete)throw new IllegalArgumentException('ISP_INVENTORY_INCOMPLETE_'+queryId);
    }
    public static void capture() {
        row(new Map<String,Object>{'kind'=>'actor','actor'=>'admin-baseline','userId'=>UserInfo.getUserId(),'profileId'=>UserInfo.getProfileId(),'isTest'=>Test.isRunningTest(),'noSetupDml'=>true});
        queryInventory('all',false);queryInventory('nonProfile',true);
        Id uid=UserInfo.getUserId();
        Integer assignmentCount=[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:uid WITH SYSTEM_MODE];
        row(new Map<String,Object>{'kind'=>'adminCount','query'=>'SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId()','count'=>assignmentCount,'assigneeId'=>uid});
        List<PermissionSetAssignment> assignments=[SELECT Id,PermissionSetId,AssigneeId,Assignee.Name FROM PermissionSetAssignment WHERE AssigneeId=:uid WITH SYSTEM_MODE ORDER BY Id LIMIT 2001];
        Boolean complete=assignments.size()<BOUND && assignments.size()==assignmentCount;
        row(new Map<String,Object>{'kind'=>'queryHeader','queryId'=>'adminAssignments','query'=>'SELECT Id, PermissionSetId, Assignee.Name FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId()','executedQuery'=>'SELECT Id,PermissionSetId,AssigneeId,Assignee.Name FROM PermissionSetAssignment WHERE AssigneeId=:uid WITH SYSTEM_MODE ORDER BY Id LIMIT 2001','reportedCount'=>assignmentCount,'complete'=>complete,'method'=>'bounded-unsaturated','bound'=>BOUND});
        for(Integer i=0;i<assignments.size();i++) {
            PermissionSetAssignment a=assignments[i];
            row(new Map<String,Object>{'kind'=>'queryRow','queryId'=>'adminAssignments','index'=>i,'record'=>new Map<String,Object>{'Id'=>a.Id,'PermissionSetId'=>a.PermissionSetId,'AssigneeId'=>a.AssigneeId,'Assignee'=>new Map<String,Object>{'Name'=>a.Assignee.Name}}});
        }
        row(new Map<String,Object>{'kind'=>'queryFooter','queryId'=>'adminAssignments','rowCount'=>assignments.size()});
        if(!complete)throw new IllegalArgumentException('ISP_ASSIGNMENTS_INCOMPLETE');
    }
    public static String finishCapture() {
        row(new Map<String,Object>{'kind'=>'limits','dmlStatements'=>Limits.getDmlStatements(),'dmlRows'=>Limits.getDmlRows(),'queries'=>Limits.getQueries(),'queryRows'=>Limits.getQueryRows(),'cpuMs'=>Limits.getCpuTime(),'heapBytes'=>Limits.getHeapSize()});
        Integer total=0;for(String value:payload){Integer n=enc(value).length();if(n>12000)throw new IllegalArgumentException('ISP_ROW_LIMIT');total+=n+100;}
        if(total>700000)throw new IllegalArgumentException('ISP_TRANSACTION_LIMIT');
        for(Integer i=0;i<payload.size();i++)System.debug(LoggingLevel.ERROR,'ISP_ROW|'+i+'|'+enc(payload[i]));
        Map<String,Object> terminal=new Map<String,Object>();terminal.putAll(submission);terminal.put('payloadRows',payload.size());terminal.put('sha256',EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256',Blob.valueOf(String.join(payload,'\n')))));
        String carrier='ISP_END|'+enc(JSON.serialize(terminal));System.debug(LoggingLevel.ERROR,carrier);return carrier;
    }
}
