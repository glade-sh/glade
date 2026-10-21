@IsTest private class EntryHookProbeTest {
    @IsTest static void typed() { System.assert(false, 'OBSERVED|typed|' + JSON.serialize(EntryHookProbe.observe('typed'))); }
    @IsTest static void erased() { System.assert(false, 'OBSERVED|erased|' + JSON.serialize(EntryHookProbe.observe('erased'))); }
    @IsTest static void emptyTyped() { System.assert(false, 'OBSERVED|emptyTyped|' + JSON.serialize(EntryHookProbe.observe('emptyTyped'))); }
    @IsTest static void emptyErased() { System.assert(false, 'OBSERVED|emptyErased|' + JSON.serialize(EntryHookProbe.observe('emptyErased'))); }
    @IsTest static void nullRows() { System.assert(false, 'OBSERVED|nullRows|' + JSON.serialize(EntryHookProbe.observe('nullRows'))); }
    @IsTest static void reflected() { System.assert(false, 'OBSERVED|reflected|' + JSON.serialize(EntryHookProbe.observe('reflected'))); }
    @IsTest static void wrongElement() { System.assert(false, 'OBSERVED|wrongElement|' + JSON.serialize(EntryHookProbe.observe('wrongElement'))); }
    @IsTest static void nonVoid() { System.assert(false, 'OBSERVED|nonVoid|' + JSON.serialize(EntryHookProbe.observe('nonVoid'))); }
}
