@isTest
global class FilesEmailTest {
  @isTest global static void emailTemplateMetadataLoadsAndSends() {
    List<EmailTemplate> templates = [SELECT Id, DeveloperName, Subject, Body, HtmlValue FROM EmailTemplate WHERE DeveloperName = 'welcome'];
    System.assertEquals(1, templates.size());
    System.assertEquals('Welcome subject', templates.get(0).Subject);
    System.assertEquals('Hello local tester', templates.get(0).Body);
    System.assertEquals('<p>Hello local tester</p>', templates.get(0).HtmlValue);

    Messaging.SingleEmailMessage rendered = Messaging.renderStoredEmailTemplate(templates.get(0).Id, null, null);
    System.assertEquals('Welcome subject', rendered.getSubject());
    System.assertEquals('Hello local tester', rendered.getPlainTextBody());
    System.assertEquals('<p>Hello local tester</p>', rendered.getHtmlBody());
    rendered.setToAddresses(new List<String>{'local@example.test'});
    rendered.setSaveAsActivity(true);

    List<Messaging.SendEmailResult> results = Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{rendered});
    System.assertEquals(1, results.size());
    System.assertEquals(1, Limits.getEmailInvocations());
  }
}
