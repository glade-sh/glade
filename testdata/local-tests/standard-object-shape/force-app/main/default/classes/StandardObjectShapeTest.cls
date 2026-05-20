@isTest
private class StandardObjectShapeTest {
  @isTest static void campaignMemberStatusShapeCompilesAndRuns() {
    Campaign campaign = new Campaign(Name = 'Spring');
    CampaignMemberStatus status = new CampaignMemberStatus(
      CampaignId = campaign.Id,
      Label = 'Responded',
      SortOrder = 1,
      IsDefault = true,
      HasResponded = true
    );

    System.assertEquals('Responded', status.Label);
    System.assertEquals(1, status.SortOrder);
    System.assertEquals(true, status.IsDefault);
    System.assertEquals(true, status.HasResponded);
  }
}
