public class RoProbe62 {
  static String ro(String o, String f) {
    Schema.SObjectType t = Schema.getGlobalDescribe().get(o);
    if (t == null) return o + '.' + f + '=noobject';
    Schema.SObjectField sf = t.getDescribe().fields.getMap().get(f);
    if (sf == null) return o + '.' + f + '=nofield';
    Integer r = sf.getDescribe().getRelationshipOrder();
    return o + '.' + f + '=' + r;
  }
  public static String run() {
    List<String> o = new List<String>();
    Integer i = 0;
    for (String p : new List<String>{'ContactPointEmail.ParentId','ContactPointAddress.ParentId','ContactPointPhone.ParentId','EntitlementContact.EntitlementId','EntitlementContact.ContactId','WorkTypeGroupMember.WorkTypeGroupId','WorkTypeGroupMember.WorkTypeId','UserListView.ListViewId','TestSuiteMembership.ApexClassId','Entitlement.AccountId','Address.ParentId','Account.ParentId','OpportunityLineItem.OpportunityId','Contact.AccountId','CaseComment.ParentId','Account.OwnerId','Contact.ReportsToId'}) {
      i++;
      o.add('RO' + i + ' ' + ro(p.substringBefore('.'), p.substringAfter('.')));
    }
    o.add('ROc1 ' + ro('RpChild__c', 'Master__c'));
    o.add('ROc2 ' + ro('RpJunc__c', 'A__c'));
    o.add('ROc3 ' + ro('RpJunc__c', 'B__c'));
    o.add('ROc4 ' + ro('RpLook__c', 'M__c'));
    return String.join(o, ' ## ');
  }
}
