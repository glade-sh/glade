public class RuntimeMessageCases {
    private static Map<String, Object> observeMessage(Boolean incoming, Boolean clientManaged, Boolean withAddress, Boolean withIds) {
        String variant = incoming == null ? 'outgoing' : (clientManaged == true ? 'managed' : 'incoming');
        if (withAddress) variant += 'address';
        if (!withIds) variant += 'noids';
        Contact recipient = new Contact(LastName='Boundary ' + variant, Email=variant + '@example.invalid');
        insert recipient;
        EmailMessage message = new EmailMessage(Subject='Local boundary', FromAddress='author@example.invalid', Status='3', MessageDate=Datetime.newInstance(2026,1,1));
        if (incoming != null) message.Incoming = incoming;
        if (clientManaged != null) message.IsClientManaged = clientManaged;
        if (withIds) message.ToIds = new String[]{String.valueOf(recipient.Id)};
        if (withAddress) message.ToAddress = recipient.Email;
        Map<String, Object> result = new Map<String, Object>();
        Database.SaveResult inserted = Database.insert(message, false);
        if (!inserted.isSuccess()) {
            result.put('insertError', String.valueOf(inserted.getErrors()[0].getStatusCode()));
            return result;
        }
        List<EmailMessageRelation> initial = [SELECT RelationId, RelationAddress FROM EmailMessageRelation WHERE EmailMessageId=:message.Id AND RelationType='ToAddress'];
        result.put('automatic', initial.size());
        if (!initial.isEmpty()) {
            result.put('automaticMatchesRecipient', initial[0].RelationId == recipient.Id);
            result.put('automaticHasNullRecipient', initial[0].RelationId == null);
            result.put('automaticMatchesAddress', initial[0].RelationAddress == recipient.Email);
        }
        Database.SaveResult saved = Database.insert(new EmailMessageRelation(EmailMessageId=message.Id,RelationId=recipient.Id,RelationType='ToAddress'), false);
        result.put('explicitSuccess', saved.isSuccess());
        if (!saved.isSuccess()) result.put('explicitError', String.valueOf(saved.getErrors()[0].getStatusCode()));
        result.put('afterExplicit', [SELECT count() FROM EmailMessageRelation WHERE EmailMessageId=:message.Id AND RelationType='ToAddress']);
        return result;
    }
    public static Map<String, Object> observe() {
        return new Map<String, Object>{'addressNoIds' => observeMessage(null,null,true,false), 'noAddressNoIds' => observeMessage(null,null,false,false), 'withAddress' => observeMessage(null,null,true,true), 'managedAddress' => observeMessage(true,true,true,true), 'outgoing' => observeMessage(null,null,false,true), 'incoming' => observeMessage(true,null,false,true), 'clientManaged' => observeMessage(true,true,false,true)};
    }
}
