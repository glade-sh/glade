@isTest
private class PresentationMetadataTest {
  @isTest static void tabsAreExplicitlyUnsupportedUntilModeled() {
    Schema.describeTabs();
  }
}
