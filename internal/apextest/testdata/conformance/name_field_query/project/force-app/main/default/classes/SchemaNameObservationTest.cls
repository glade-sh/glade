@IsTest
private class SchemaNameObservationTest {
    @IsTest static void accountNameQuery() { observeQuery('accountNameQuery', 'Account'); }
    @IsTest static void caseNameQuery() { observeQuery('caseNameQuery', 'Case'); }

    private static void observeQuery(String method, String objectName) {
        List<String> objectNames = new List<String>{objectName};
        List<FieldDefinition> fields = [
            SELECT QualifiedApiName, Label
            FROM FieldDefinition
            WHERE EntityDefinition.QualifiedApiName IN :objectNames AND IsNameField = TRUE
            LIMIT 1
        ];
        Map<String,Object> observation = new Map<String,Object>{'count'=>fields.size()};
        if (!fields.isEmpty()) {
            observation.put('name',fields[0].QualifiedApiName);
            observation.put('label',fields[0].Label);
        }
        emit(method,observation);
    }

    @IsTest static void describeNameControl() {
        emit('describeNameControl',new Map<String,Object>{
            'accountName'=>Account.Name.getDescribe().isNameField(),
            'accountIndustry'=>Account.Industry.getDescribe().isNameField(),
            'caseNumber'=>Case.CaseNumber.getDescribe().isNameField(),
            'caseSubject'=>Case.Subject.getDescribe().isNameField()
        });
    }

    private static void emit(String method, Map<String,Object> observation) {
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(observation));
    }
}
