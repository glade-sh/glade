@IsTest
private class CommonProjectShapesTest {
    @IsTest static void topLevel() {
        System.assert(false, 'OBSERVED|topLevel|' + JSON.serialize(new Map<String, Object>{'value' => new ShapeUse().topLevelRoute()}));
    }
    @IsTest static void nested() {
        System.assert(false, 'OBSERVED|nested|' + JSON.serialize(new Map<String, Object>{'value' => new ShapeBase().nestedRoute()}));
    }
    @IsTest static void inheritedLocal() {
        System.assert(false, 'OBSERVED|inheritedLocal|' + JSON.serialize(new Map<String, Object>{'value' => new ShapeDerived().inheritedLocalRoute()}));
    }
    @IsTest static void inheritedField() {
        System.assert(false, 'OBSERVED|inheritedField|' + JSON.serialize(new Map<String, Object>{'value' => new ShapeDerived().inheritedFieldRoute()}));
    }
}
