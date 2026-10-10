@IsTest
private class OverloadApplicabilityTest {
    private virtual class BaseValue {}
    private virtual class NarrowValue extends BaseValue {
        public BaseValue held;
        public String chooseHeld() {
            held = new LeafValue();
            return new OverloadTarget().choose(this, held);
        }
    }
    private class LeafValue extends NarrowValue {}
    private class OverloadTarget {
        public String bare(Id value, Map<Id, List<String>> rows) { return 'id-map'; }
        public String bare(String value, NarrowValue rows) { return 'string-class'; }
        public String choose(NarrowValue left, LeafValue right) { return 'narrow-leaf'; }
        public String choose(BaseValue left, BaseValue right) { return 'base-base'; }
        public String nullOnly(Map<Id, List<String>> rows) { return 'map'; }
        public String nullOnly(NarrowValue rows) { return 'class'; }
        public String sequence(Object value) { return 'Object'; }
        public String sequence(List<String> value) { return 'List<String>'; }
    }

    @IsTest static void idAndBareNull() {
        Id value = '001000000000001';
        System.assertEquals('id-map', new OverloadTarget().bare(value, null));
    }
    @IsTest static void stringAndBareNull() {
        String value = '001000000000001';
        System.assertEquals('string-class', new OverloadTarget().bare(value, null));
    }
    @IsTest static void declaredBaseWithLeafValue() {
        NarrowValue left = new NarrowValue();
        BaseValue right = new LeafValue();
        System.assertEquals('base-base', new OverloadTarget().choose(left, right));
    }
    @IsTest static void declaredBaseWithNullValue() {
        NarrowValue left = new NarrowValue();
        BaseValue right = null;
        System.assertEquals('base-base', new OverloadTarget().choose(left, right));
    }
    @IsTest static void explicitLeafCast() {
        NarrowValue left = new NarrowValue();
        BaseValue right = new LeafValue();
        System.assertEquals('narrow-leaf', new OverloadTarget().choose(left, (LeafValue)right));
    }
    @IsTest static void declaredThisAndBaseField() {
        System.assertEquals('base-base', new NarrowValue().chooseHeld());
    }
    @IsTest static void typedMapNull() {
        Map<Id, List<String>> rows = null;
        System.assertEquals('map', new OverloadTarget().nullOnly(rows));
    }
    @IsTest static void typedListNull() {
        List<String> value = null;
        System.assertEquals('List<String>', new OverloadTarget().sequence(value));
    }
}
