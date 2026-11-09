Map<String,Schema.SObjectType> g = Schema.getGlobalDescribe();
Map<String,Object> m = new Map<String,Object>();
for (String n : new List<String>{'AppointmentCategory','AppointmentInvitation','AssessmentIndicatorDefinition','AssessmentTask','AssessmentTaskContentDocument','AssessmentTaskDefinition','AssessmentTaskOrder','ChangeRequest','Coupon','DelegatedAccount','DeliveryEstimationSetup','DevopsActivityLog','DevopsEnvironment','DevopsRequestInfo','DocumentRecipient','FulfillmentOrder','GenericVisitTask','GenericVisitTaskContext','GnrcVstKeyPerformanceInd','GnrcVstTaskContextRelation','Incident','LocationShippingCarrierMethod','OrderSummary','PickTicket','PickTicketAssignment','Problem','ProductCatalog','Promotion','PromotionMarketSegment','PromotionQualifier','PromotionSegment','PromotionSegmentBuyerGroup','PromotionSegmentSalesStore','PromotionTarget','PromotionTier','Quote','ShippingCarrier','ShippingCarrierMethod','SurveyQuestion','UserLocationAssignment','Visit','VisitedParty','Waitlist','WaitlistParticipant','PrivacyRTBFRequest'}) {
  Boolean inG = g.containsKey(n);
  String act = 'absent';
  if (inG) { Schema.DescribeSObjectResult d = g.get(n).getDescribe(); act = 'present'; }
  m.put(n, act);
}
System.debug('OBS|' + JSON.serialize(m));
