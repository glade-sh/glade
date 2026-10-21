@IsTest
private class DefaultCursorObservationTest {
    @IsTest static void directCreatedCursor() { observe('directCreatedCursor',false); }
    @IsTest static void jsonCreatedCursor() { observe('jsonCreatedCursor',true); }
    private static void observe(String method, Boolean encode) {
        List<Account> source = new List<Account>{new Account(Name='North'),new Account(Name='West'),new Account(Name='South')};
        insert source;
        List<Account> page = [SELECT Id,CreatedDate FROM Account ORDER BY CreatedDate DESC,Id DESC LIMIT 1];
        Datetime cursorStamp = page[0].CreatedDate;
        Id cursorId = page[0].Id;
        if (encode) {
            Map<String,Object> payload = new Map<String,Object>{'stamp'=>cursorStamp.getTime(),'key'=>String.valueOf(cursorId)};
            String token = EncodingUtil.base64Encode(Blob.valueOf(JSON.serialize(payload)));
            Map<String,Object> decoded = (Map<String,Object>)JSON.deserializeUntyped(EncodingUtil.base64Decode(token).toString());
            cursorStamp = Datetime.newInstance(Long.valueOf(String.valueOf(decoded.get('stamp'))));
            cursorId = (Id)String.valueOf(decoded.get('key'));
        }
        List<Account> next = Database.queryWithBinds(
            'SELECT Id,CreatedDate FROM Account WHERE (CreatedDate < :stamp OR (CreatedDate = :stamp AND Id < :key)) ORDER BY CreatedDate DESC,Id DESC',
            new Map<String,Object>{'stamp'=>cursorStamp,'key'=>cursorId},AccessLevel.SYSTEM_MODE
        );
        Boolean repeats = new Map<Id,Account>(next).containsKey(cursorId);
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(new Map<String,Object>{'remaining'=>next.size(),'repeatsCursor'=>repeats,'storedFraction'=>Math.mod(page[0].CreatedDate.getTime(),1000L),'cursorFraction'=>Math.mod(cursorStamp.getTime(),1000L),'cursorMillisDelta'=>cursorStamp.getTime()-page[0].CreatedDate.getTime(),'cursorMatches'=>cursorId==page[0].Id,'olderOnly'=>[SELECT Count() FROM Account WHERE CreatedDate < :cursorStamp],'equalStamp'=>[SELECT Count() FROM Account WHERE CreatedDate = :cursorStamp],'lowerIdOnly'=>[SELECT Count() FROM Account WHERE Id < :cursorId],'sameId'=>[SELECT Count() FROM Account WHERE Id = :cursorId]}));
    }
}
