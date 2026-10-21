List<Object> answers = new List<Object>();
for (String textValue : new List<String>{'001000000000009', '999000000000009', 'zzz000000000009', '001', '001000000000009000000000', ''}) {
    Map<String,Object> row = new Map<String,Object>{'input'=>textValue};
    String stage = 'assignment';
    try {
        Id token = textValue;
        row.put('assigned', String.valueOf(token));
        stage = 'describe';
        row.put('type', String.valueOf(token.getSObjectType()));
    } catch (Exception error) {
        row.put('stage', stage);
        row.put('error', error.getTypeName());
        row.put('message', error.getMessage());
    }
    answers.add(row);
}
Map<String,Object> setRow = new Map<String,Object>{'input'=>'set-add-invalid'};
try {
    Set<Id> tokens = new Set<Id>();
    tokens.add('not-an-id');
    setRow.put('size', tokens.size());
} catch (Exception error) {
    setRow.put('error', error.getTypeName());
    setRow.put('message', error.getMessage());
}
answers.add(setRow);
System.debug('OBS|' + JSON.serialize(answers));
