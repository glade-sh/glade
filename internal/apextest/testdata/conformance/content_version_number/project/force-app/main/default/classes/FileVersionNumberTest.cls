@IsTest
private class FileVersionNumberTest {
    @IsTest static void initialUpload() {
        ContentVersion draft = makeVersion(null, 'first');
        insert draft;
        ContentVersion saved = [SELECT VersionNumber, IsLatest FROM ContentVersion WHERE Id = :draft.Id];
        observe('initialUpload', new Map<String, Object>{
            'number' => saved.VersionNumber,
            'latest' => saved.IsLatest
        });
    }
    @IsTest static void secondUpload() {
        ContentVersion first = makeVersion(null, 'first');
        insert first;
        first = [SELECT Id, ContentDocumentId FROM ContentVersion WHERE Id = :first.Id];
        ContentVersion second = makeVersion(first.ContentDocumentId, 'second');
        insert second;
        first = [SELECT VersionNumber, IsLatest FROM ContentVersion WHERE Id = :first.Id];
        second = [SELECT VersionNumber, IsLatest FROM ContentVersion WHERE Id = :second.Id];
        observe('secondUpload', new Map<String, Object>{
            'firstNumber' => first.VersionNumber,
            'firstLatest' => first.IsLatest,
            'secondNumber' => second.VersionNumber,
            'secondLatest' => second.IsLatest
        });
    }
    private static ContentVersion makeVersion(Id documentId, String body) {
        ContentVersion version = new ContentVersion(
            Title = 'Example',
            PathOnClient = 'example.txt',
            VersionData = Blob.valueOf(body),
            IsMajorVersion = false
        );
        if (documentId != null) version.ContentDocumentId = documentId;
        return version;
    }
    private static void observe(String label, Map<String, Object> result) {
        System.assert(false, 'OBSERVED|' + label + '|' + JSON.serialize(result));
    }
}
