public class RuntimeMessageCases {
    private static Map<String,Object> one(String mode, String operation) {
        Contact recipient = new Contact(LastName='Recipient '+mode+' '+operation,Email=mode+'.'+operation+'@example.invalid'); insert recipient;
        EmailMessage message = new EmailMessage(Subject='Boundary',FromAddress='author@example.invalid',Status='3');
        if (mode == 'outgoing' || mode == 'incoming' || mode == 'managed') message.ToIds = new String[]{String.valueOf(recipient.Id)};
        if (mode == 'incoming' || mode == 'managed') message.Incoming = true;
        if (mode == 'managed') message.IsClientManaged = true;
        if (mode == 'stringList') message.ToIds = new List<String>{String.valueOf(recipient.Id)};
        if (mode == 'idArray') message.ToIds = new Id[]{recipient.Id};
        if (mode == 'empty') message.ToIds = new List<String>();
        if (mode == 'null') message.ToIds = null;
        Map<String,Object> result = new Map<String,Object>();
        Boolean success = false;
        try {
            if (operation == 'statement') { insert message; success=true; }
            else if (operation == 'upsert') {
                Database.UpsertResult saved=Database.upsert(message,false); success=saved.isSuccess();
                if (!success) {result.put('error',String.valueOf(saved.getErrors()[0].getStatusCode()));result.put('message',saved.getErrors()[0].getMessage());result.put('fields',saved.getErrors()[0].getFields());}
            } else {
                Database.SaveResult saved;
                if (operation == 'list') saved=Database.insert(new List<EmailMessage>{message},false)[0];
                else saved=Database.insert(message,operation == 'all');
                success=saved.isSuccess();
                if (!success) {result.put('error',String.valueOf(saved.getErrors()[0].getStatusCode()));result.put('message',saved.getErrors()[0].getMessage());result.put('fields',saved.getErrors()[0].getFields());}
            }
        } catch(DmlException e) { result.put('error',String.valueOf(e.getDmlStatusCode(0)));result.put('message',e.getDmlMessage(0));result.put('fields',e.getDmlFieldNames(0)); }
        result.put('success',success);result.put('hasId',message.Id != null);
        if(success) {
            result.put('automatic',[SELECT count() FROM EmailMessageRelation WHERE EmailMessageId=:message.Id AND RelationType='ToAddress']);
            Database.SaveResult explicit=Database.insert(new EmailMessageRelation(EmailMessageId=message.Id,RelationId=recipient.Id,RelationType='ToAddress'),false);
            result.put('explicitSuccess',explicit.isSuccess());
            result.put('afterExplicit',[SELECT count() FROM EmailMessageRelation WHERE EmailMessageId=:message.Id AND RelationType='ToAddress']);
        }
        return result;
    }
    private static Map<String,Object> mixed() {
        Contact recipient = new Contact(LastName='Mixed recipient',Email='mixed@example.invalid'); insert recipient;
        EmailMessage first = new EmailMessage(Subject='First',FromAddress='author@example.invalid',Status='3');
        first.ToIds=new String[]{String.valueOf(recipient.Id)};
        EmailMessage second = new EmailMessage(Subject='Second',FromAddress='author@example.invalid',Status='3');
        List<Database.SaveResult> saved=Database.insert(new List<EmailMessage>{first,second},false);
        return new Map<String,Object>{'successes'=>new List<Boolean>{saved[0].isSuccess(),saved[1].isSuccess()},'hasIds'=>new List<Boolean>{first.Id!=null,second.Id!=null}};
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result=new Map<String,Object>();
        for(String operation:new String[]{'partial','all','list','statement','upsert'}) {
            List<String> modes = operation == 'partial' ? new String[]{'outgoing','incoming','managed','stringList','idArray','empty','null','unset'} : new String[]{'outgoing'};
            for(String mode:modes) result.put(operation+'/'+mode,one(mode,operation));
        }
        result.put('mixedList',mixed());
        return result;
    }
}
