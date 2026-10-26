@IsTest
private class HostedBoundaryProbe67 {
    @IsTest static void blobToPdf() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = Blob.toPdf('Synthetic boundary input.'); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('blobToPdf', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void pageGetContentAsPdf() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = new PageReference('/apex/HostedBoundaryProbe67').getContentAsPDF(); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('pageGetContentAsPdf', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void customNotificationSend() {
        Id typeId = [SELECT Id FROM CustomNotificationType WHERE DeveloperName = 'Hosted_Boundary_Probe' LIMIT 1].Id;
        Messaging.CustomNotification notice = new Messaging.CustomNotification();
        notice.setNotificationTypeId(typeId); notice.setTitle('Synthetic title'); notice.setBody('Synthetic body'); notice.setTargetId(UserInfo.getUserId());
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { notice.send(new Set<String>{UserInfo.getUserId()}); returned = true; resultType = 'void'; resultValue = null; }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('customNotificationSend', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void appAnalytics() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { IsvPartners.AppAnalytics.logCustomInteraction(HostedBoundaryLabels67.Interaction.Opened); returned = true; resultType = 'void'; resultValue = null; }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('appAnalytics', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void route_constructor_content() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = new PageReference('/apex/HostedBoundaryProbe67').getContent(); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('route_constructor_content', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void route_constructor_pdf() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = new PageReference('/apex/HostedBoundaryProbe67').getContentAsPDF(); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('route_constructor_pdf', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void route_page_constant_content() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = Page.HostedBoundaryProbe67.getContent(); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('route_page_constant_content', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void route_page_constant_pdf() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = Page.HostedBoundaryProbe67.getContentAsPDF(); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('route_page_constant_pdf', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void route_helper_constructor_content() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = HostedPageRenderer67.render(new PageReference('/apex/HostedBoundaryProbe67'), false); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('route_helper_constructor_content', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void route_helper_constructor_pdf() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = HostedPageRenderer67.render(new PageReference('/apex/HostedBoundaryProbe67'), true); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('route_helper_constructor_pdf', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void route_helper_page_constant_content() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = HostedPageRenderer67.render(Page.HostedBoundaryProbe67, false); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('route_helper_page_constant_content', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    @IsTest static void route_helper_page_constant_pdf() {
        Integer c = Limits.getCallouts(), q = Limits.getQueries(), d = Limits.getDmlStatements(), e = Limits.getEmailInvocations(), cpu = Limits.getCpuTime(), heap = Limits.getHeapSize();
        Boolean returned = false; String resultType; String resultValue; String errorType; String errorMessage;
        try { Blob result = HostedPageRenderer67.render(Page.HostedBoundaryProbe67, true); returned = true; resultType = result == null ? 'null' : 'Blob'; resultValue = result == null ? null : String.valueOf(result.size()); }
        catch (Exception ex) { errorType = ex.getTypeName(); errorMessage = ex.getMessage(); }
        record('route_helper_page_constant_pdf', returned, resultType, resultValue, errorType, errorMessage, c, q, d, e, cpu, heap);
    }

    private static void record(String operation, Boolean returned, String resultType, String resultValue, String errorType, String errorMessage, Integer c, Integer q, Integer d, Integer e, Integer cpu, Integer heap) {
        Map<String,Object> observation = new Map<String,Object>{'returned' => returned, 'returnType' => resultType, 'returnValue' => resultValue, 'exceptionType' => errorType, 'exceptionMessage' => errorMessage, 'callouts' => Limits.getCallouts() - c, 'queries' => Limits.getQueries() - q, 'dml' => Limits.getDmlStatements() - d, 'emails' => Limits.getEmailInvocations() - e, 'cpuMs' => Limits.getCpuTime() - cpu, 'heapBytes' => Limits.getHeapSize() - heap};
        System.assert(false, 'HB|' + operation + '|' + JSON.serialize(observation));
    }
}
