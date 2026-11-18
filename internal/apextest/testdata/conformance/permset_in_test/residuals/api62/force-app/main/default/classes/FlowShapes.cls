@IsTest private class FlowShapes {
 static String run(String n) { try { Flow.Interview f = Flow.Interview.createInterview(n, new Map<String,Object>()); f.start(); return 'ok:' + String.valueOf(f.getVariableValue('Out')); } catch (Exception e) { return 'ex:' + e.getTypeName() + ':' + e.getMessage(); } }
 @IsTest static void t() {
  String p = 'flows';
  for (String n : new List<String>{'Fl_exact','Fl_prefix','Fl_plain','Fl_var','Fl_ref'}) p += '|' + n + '=' + run(n);
  insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = PsrUtil.permSetId());
  for (String n : new List<String>{'Fl_exact','Fl_prefix','Fl_plain','Fl_var','Fl_ref'}) p += '|after_' + n + '=' + run(n);
  System.assert(false, p);
 }
}
