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

  @isTest global static void fileObjectsRoundTripAndVersion() {
    String ownerId = '005000000000001';

    Attachment attachment = new Attachment(Name = 'note.txt', ParentId = ownerId, Body = Blob.valueOf('attachment body'), ContentType = 'text/plain');
    insert attachment;
    Attachment storedAttachment = [SELECT Id, ParentId, Body, ContentType FROM Attachment WHERE Id = :attachment.Id];
    System.assertEquals(ownerId, storedAttachment.ParentId);
    System.assertEquals('attachment body', storedAttachment.Body.toString());
    System.assertEquals('text/plain', storedAttachment.ContentType);

    Document document = new Document(Name = 'Terms.pdf', DeveloperName = 'Terms', Body = Blob.valueOf('document body'), ContentType = 'application/pdf', Type = 'pdf', IsPublic = true);
    insert document;
    Document storedDocument = [SELECT Id, Body, ContentType, Type, IsPublic FROM Document WHERE Id = :document.Id];
    System.assertEquals('document body', storedDocument.Body.toString());
    System.assertEquals('application/pdf', storedDocument.ContentType);
    System.assertEquals('pdf', storedDocument.Type);
    System.assert(storedDocument.IsPublic);

    ContentVersion first = new ContentVersion(Title = 'Spec', PathOnClient = 'spec.txt', VersionData = Blob.valueOf('first'), FirstPublishLocationId = ownerId);
    insert first;
    ContentVersion storedFirst = [SELECT Id, ContentDocumentId, VersionData, IsLatest FROM ContentVersion WHERE Id = :first.Id];
    System.assertEquals('first', storedFirst.VersionData.toString());
    Id documentId = storedFirst.ContentDocumentId;

    ContentVersion second = new ContentVersion(Title = 'Spec v2', PathOnClient = 'spec-v2.txt', VersionData = EncodingUtil.base64Decode('c2Vjb25k'), ContentDocumentId = documentId);
    insert second;

    List<ContentVersion> versions = [SELECT Id, IsLatest FROM ContentVersion WHERE ContentDocumentId = :documentId];
    System.assertEquals(2, versions.size());
    Integer latestCount = 0;
    for (ContentVersion version : versions) {
      if (version.IsLatest) {
        latestCount++;
        System.assertEquals(second.Id, version.Id);
      }
    }
    System.assertEquals(1, latestCount);

    ContentDocument contentDocument = [SELECT Id, Title, LatestPublishedVersionId FROM ContentDocument WHERE Id = :documentId];
    System.assertEquals('Spec v2', contentDocument.Title);
    System.assertEquals(second.Id, contentDocument.LatestPublishedVersionId);

    List<ContentDocumentLink> links = [SELECT Id, ContentDocumentId, LinkedEntityId FROM ContentDocumentLink WHERE ContentDocumentId = :documentId];
    System.assertEquals(1, links.size());
    System.assertEquals(ownerId, links.get(0).LinkedEntityId);
  }
}
