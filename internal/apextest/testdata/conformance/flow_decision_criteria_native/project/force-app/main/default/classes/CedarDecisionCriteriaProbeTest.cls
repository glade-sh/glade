@IsTest
private class CedarDecisionCriteriaProbeTest {
    @IsTest static void observe() {
        insert new List<Contact>{
            new Contact(LastName = 'TitleContains', Title = 'Senior Guide'),
            new Contact(LastName = 'TitleStartsWith', Title = 'Guide Specialist'),
            new Contact(LastName = 'TitleEndsWith', Title = 'Senior Guide'),
            new Contact(LastName = 'TitleDoesNotEqual', Title = 'Guide'),
            new Contact(LastName = 'TitleIsNotNull', Title = 'Guide'),
            new Contact(LastName = 'TitleNullField'),
            new Contact(LastName = 'TitleIsBlank', Title = '   '),
            new Contact(LastName = 'TitleWhitespaceIsNull', Title = '   '),
            new Contact(LastName = 'ScalarContains', Title = 'Senior Guide'),
            new Contact(LastName = 'ScalarIsNotNull', Title = 'Guide'),
            new Contact(LastName = 'RecordIsNotNull', Title = 'Guide'),
            new Contact(LastName = 'RecordFieldEqualsScalar', Title = 'Guide'),
            new Contact(LastName = 'ExpectedValue', Title = 'Guide'),
            new Contact(LastName = 'CollectionNotEmpty'),
            new Contact(LastName = 'CollectionIsNotNull'),
            new Contact(LastName = 'AfterSaveTitleContains', Title = 'Senior Guide'),
            new Contact(LastName = 'VariableEqualCase', Title = 'guide'),
            new Contact(LastName = 'VariableNotEqualCase', Title = 'guide'),
            new Contact(LastName = 'VariableEqualTrim', Title = ' Guide '),
            new Contact(LastName = 'VariableNotEqualTrim', Title = ' Guide '),
            new Contact(LastName = 'VariableIsNullWhitespace', Title = '   '),
            new Contact(LastName = 'TitleContainsNoMatch', Title = 'Senior Guide'),
            new Contact(LastName = 'TitleEqualCaseLiteral', Title = 'guide'),
            new Contact(LastName = 'TitleEqualTrimLiteral', Title = ' Guide '),
            new Contact(LastName = 'TitleNotEqualTrimLiteral', Title = ' Guide '),
            new Contact(LastName = 'VariableIsBlankWhitespace', Title = '   '),
            new Contact(LastName = 'DeleteLookupTitleContains', Title = 'Senior Guide'),
            new Contact(LastName = 'CedarExpected', Title = 'Guide'),
            new Contact(LastName = 'CedarAutolaunched', Title = 'Senior Guide')
        };

        List<String> names = new List<String>{
            'TitleContains', 'TitleStartsWith', 'TitleEndsWith', 'TitleDoesNotEqual',
            'TitleIsNull', 'TitleIsNotNull', 'TitleNullField', 'TitleIsBlank',
            'TitleWhitespaceIsNull', 'ScalarContains', 'ScalarIsNull',
            'ScalarIsNotNull', 'RecordIsNull', 'RecordIsNotNull',
            'RecordFieldEqualsScalar', 'CollectionIsEmpty', 'CollectionNotEmpty',
            'CollectionIsNull', 'CollectionIsNotNull', 'AfterSaveTitleContains',
            'VariableEqualCase', 'VariableNotEqualCase', 'VariableEqualTrim',
            'VariableNotEqualTrim', 'VariableIsNullWhitespace',
            'LookupNoMatchIsNullTrue', 'LookupNoMatchIsNullFalse', 'TitleContainsNoMatch',
            'TitleEqualCaseLiteral', 'TitleEqualTrimLiteral', 'TitleNotEqualTrimLiteral',
            'VariableIsBlankWhitespace'
        };
        Map<String, Object> observed = new Map<String, Object>();
        Contact whitespaceTitle = [SELECT Title FROM Contact WHERE LastName = 'VariableIsNullWhitespace' LIMIT 1];
        observed.put('TitleWhitespaceFieldReadback', whitespaceTitle.Title == null ? 'null' : 'length=' + String.valueOf(whitespaceTitle.Title.length()));
        for (String name : names) {
            try {
                Account row = new Account(Name = name);
                insert row;
                row = [SELECT Description FROM Account WHERE Id = :row.Id];
                observed.put(name, row.Description);
            } catch (Exception e) {
                observed.put(name, 'ERROR|' + e.getTypeName() + '|' + e.getMessage());
            }
        }
        observed.put('AutoLookupTitleContains', runFlow('Cedar_AutoLookupTitleContains', new Map<String, Object>{'lookupKey' => 'CedarAutolaunched'}));
        observed.put('AutoVariableEqualCase', runFlow('Cedar_AutoVariableEqualCase', new Map<String, Object>{'leftValue' => 'guide', 'rightValue' => 'Guide'}));
        observed.put('AutoVariableNotEqualCase', runFlow('Cedar_AutoVariableNotEqualCase', new Map<String, Object>{'leftValue' => 'guide', 'rightValue' => 'Guide'}));
        observed.put('AutoVariableEqualTrim', runFlow('Cedar_AutoVariableEqualTrim', new Map<String, Object>{'leftValue' => ' Guide ', 'rightValue' => 'Guide'}));
        observed.put('AutoVariableNotEqualTrim', runFlow('Cedar_AutoVariableNotEqualTrim', new Map<String, Object>{'leftValue' => ' Guide ', 'rightValue' => 'Guide'}));
        observed.put('AutoVariableIsNullWhitespace', runFlow('Cedar_AutoVariableIsNullWhitespace', new Map<String, Object>{'leftValue' => '   '}));
        observed.put('AutoVariableIsBlankWhitespace', runFlow('Cedar_AutoVariableIsBlankWhitespace', new Map<String, Object>{'leftValue' => '   '}));
        try {
            Account row = new Account(Name = 'DeleteLookupTitleContains');
            insert row;
            delete row;
            Contact result = [SELECT Title FROM Contact WHERE LastName = 'CedarDeleteResult' LIMIT 1];
            observed.put('DeleteLookupTitleContains', result.Title);
        } catch (Exception e) {
            observed.put('DeleteLookupTitleContains', 'ERROR|' + e.getTypeName() + '|' + e.getMessage());
        }
        System.assert(false, 'OBSERVED|decisionCriteria|' + JSON.serialize(observed));
    }

    private static Object runFlow(String flowName, Map<String, Object> inputs) {
        try {
            Flow.Interview interview = Flow.Interview.createInterview(flowName, inputs);
            interview.start();
            return interview.getVariableValue('outcome');
        } catch (Exception e) {
            return 'ERROR|' + e.getTypeName() + '|' + e.getMessage();
        }
    }
}
