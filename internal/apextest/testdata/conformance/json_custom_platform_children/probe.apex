Map<String, Object> rows = new Map<String, Object>();
QuartzNote__c parent = new QuartzNote__c(Name='Quartz');
insert parent;
Id parentId = parent.Id;
for (SObject seed : new List<SObject>{
    new Attachment(ParentId=parentId, Name='Quartz.txt', Body=Blob.valueOf('Quartz note')),
    new Note(ParentId=parentId, Title='Quartz note', Body='A note'),
    new ContentVersion(Title='Quartz', PathOnClient='Quartz.txt',
        VersionData=Blob.valueOf('Quartz file'), FirstPublishLocationId=parentId)
}) {
    try {
        insert seed;
    } catch (Exception e) {
        rows.put('seed_' + String.valueOf(seed.getSObjectType()), new List<String>{e.getTypeName(), e.getMessage()});
    }
}
Map<String, String> inputs = new Map<String, String>();
for (String child : new List<String>{
    'ContentDocumentLinks', 'Attachments', 'Notes', 'NotesAndAttachments',
    'CombinedAttachments', 'AttachedContentDocuments'
}) {
    try {
        SObject queried = Database.query('SELECT Name,(SELECT Id FROM ' + child +
            ') FROM QuartzNote__c WHERE Id = :parentId');
        List<SObject> queriedChildren = queried.getSObjects(child);
        rows.put(child + '_soql', queriedChildren == null ? null : queriedChildren.size());
        inputs.put(child, JSON.serialize(queried));
    } catch (Exception e) {
        rows.put(child + '_soql', new List<String>{e.getTypeName(), e.getMessage()});
    }
}
String empty = '{"attributes":{"type":"QuartzNote__c"},"ContentDocumentLinks":{"totalSize":0,"done":true,"records":[]}}';
inputs.put('empty_links', empty);
inputs.put('lower_links', empty.replace('ContentDocumentLinks', 'contentdocumentlinks'));
if (inputs.containsKey('ContentDocumentLinks')) {
    inputs.put('lower_populated_links', inputs.get('ContentDocumentLinks')
        .replace('ContentDocumentLinks', 'contentdocumentlinks'));
}

inputs.put('unknown_empty', empty.replace('ContentDocumentLinks', 'MissingRelationship'));
inputs.put('naked_links', '{"attributes":{"type":"QuartzNote__c"},"ContentDocumentLinks":[]}');
inputs.put('invalid_links', '{"attributes":{"type":"QuartzNote__c"},"ContentDocumentLinks":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"Note"},"Title":"Quartz"}]}}');
for (String kind : new List<String>{'CombinedAttachment', 'NoteAndAttachment'}) {
    inputs.put('manual_' + kind, '{"attributes":{"type":"QuartzNote__c"},"' +
        (kind == 'CombinedAttachment' ? 'CombinedAttachments' : 'NotesAndAttachments') +
        '":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"' + kind + '"}}]}}');
}
Map<String, String> accessors = new Map<String, String>{
    'empty_links' => 'ContentDocumentLinks', 'lower_links' => 'ContentDocumentLinks',
    'lower_populated_links' => 'ContentDocumentLinks',
    'unknown_empty' => 'MissingRelationship', 'invalid_links' => 'ContentDocumentLinks',
    'naked_links' => 'ContentDocumentLinks',
    'manual_CombinedAttachment' => 'CombinedAttachments',
    'manual_NoteAndAttachment' => 'NotesAndAttachments'
};
for (String key : inputs.keySet()) {
    String child = accessors.containsKey(key) ? accessors.get(key) : key;
    for (String mode : new List<String>{'lax', 'strict', 'generic'}) {
        String slot = key + '_' + mode;
        try {
            String input = inputs.get(key);
            SObject decoded = (SObject)(mode == 'strict' ? JSON.deserializeStrict(input, QuartzNote__c.class)
                : JSON.deserialize(input, mode == 'generic' ? SObject.class : QuartzNote__c.class));
            Map<String, Object> serialized = (Map<String, Object>)JSON.deserializeUntyped(JSON.serialize(decoded));
            rows.put(slot + '_serialized', serialized.containsKey(child));
            try {
                List<SObject> children = decoded.getSObjects(child);
                rows.put(slot, new List<Object>{
                    children == null ? null : children.size(),
                    children != null && !children.isEmpty() ? String.valueOf(children[0].getSObjectType()) : null
                });
            } catch (Exception e) {
                rows.put(slot, new List<String>{'access', e.getTypeName(), e.getMessage()});
            }
        } catch (Exception e) {
            rows.put(slot, new List<String>{'deserialize', e.getTypeName(), e.getMessage()});
        }
    }
}
