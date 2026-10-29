@IsTest private class CedarRouteProbeTest {
    @IsTest static void AfterFlowInsert() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('after-flow','insert')); }
    @IsTest static void AfterFlowUpdate() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('after-flow','update')); }
    @IsTest static void BeforeFlowInsert() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('before-flow','insert')); }
    @IsTest static void BeforeFlowUpdate() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('before-flow','update')); }
    @IsTest static void BeforeInvocableInsert() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('before-invocable','insert')); }
    @IsTest static void BeforeInvocableUpdate() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('before-invocable','update')); }
    @IsTest static void AfterInvocableInsert() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('after-invocable','insert')); }
    @IsTest static void AfterInvocableUpdate() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('after-invocable','update')); }
    @IsTest static void BeforeInterviewInsert() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('before-interview','insert')); }
    @IsTest static void BeforeInterviewUpdate() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('before-interview','update')); }
    @IsTest static void AfterInterviewInsert() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('after-interview','insert')); }
    @IsTest static void AfterInterviewUpdate() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('after-interview','update')); }
    @IsTest static void InvocableInsert() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('invocable','insert')); }
    @IsTest static void InvocableUpdate() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('invocable','update')); }
    @IsTest static void InterviewInsert() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('interview','insert')); }
    @IsTest static void InterviewUpdate() { System.assert(false, 'CEDAR_RESULT|' + CedarRouteProbe.observe('interview','update')); }
}
