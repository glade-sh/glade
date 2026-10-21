@IsTest private class SproutScalarProof {
    @IsTest static void dynamicOutput() {
        Flow.Interview i = Flow.Interview.createInterview('SproutScalar', new Map<String,Object>{'seed'=>'leaf'});
        i.start();
        System.assertEquals('leaf', i.getVariableValue('result'));
    }
    @IsTest static void subclassInputConversion() {
        Flow.Interview i = new Flow.Interview.SproutScalar(new Map<String,Object>{'seed'=>42});
        i.start();
        System.assertEquals('42', i.getVariableValue('result'));
    }
    @IsTest static void absentVariable() {
        Flow.Interview i = Flow.Interview.createInterview('SproutScalar', new Map<String,Object>{'seed'=>'leaf','absent'=>'extra'});
        i.start();
        System.assertEquals(null, i.getVariableValue('absent'));
    }
}
