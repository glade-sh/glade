@IsTest
private class Q3ActTest {
    @IsTest static void prefixed() {
        Flow.Interview i = Flow.Interview.createInterview('Q3FlowPrefixed', new Map<String,Object>());
        i.start();
        System.assertEquals(true, i.getVariableValue('ok'), 'prefixed: ok=' + i.getVariableValue('ok') + ' msg=' + i.getVariableValue('msg'));
        System.assertEquals('hi bob', i.getVariableValue('msg'));
    }
}
