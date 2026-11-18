@IsTest
private class PsrVfTest {
    @IsTest
    static void visualforcePermission() {
        Id psId = PsrUtil.permSetId();
        String ins = 'ok';
        try {
            insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
        } catch (Exception e) {
            ins = 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }

        String payload = 'setCurrentPage=start';
        String render = 'notAttempted';
        try {
            Test.setCurrentPage(Page.PsrPage);
            payload = 'setCurrentPage=ok;page=' + String.valueOf(ApexPages.currentPage());
        } catch (Exception e) {
            payload = 'setCurrentPage=ex:' + e.getTypeName() + ':' + e.getMessage();
        }
        try {
            PageReference pr = Page.PsrPage;
            Blob b = pr.getContent();
            render = 'ok:len=' + (b == null ? -1 : b.size());
            if (b != null) {
                render += ':body=' + b.toString();
            }
        } catch (Exception e) {
            render = 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }
        System.assert(false, 'insert=' + ins + ';' + payload + ';render=' + render);
    }
}
