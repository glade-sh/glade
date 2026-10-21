System.debug('OBSERVED|typed|' + JSON.serialize(EntryHookProbe.observe('typed')));
System.debug('OBSERVED|erased|' + JSON.serialize(EntryHookProbe.observe('erased')));
System.debug('OBSERVED|emptyTyped|' + JSON.serialize(EntryHookProbe.observe('emptyTyped')));
System.debug('OBSERVED|emptyErased|' + JSON.serialize(EntryHookProbe.observe('emptyErased')));
System.debug('OBSERVED|nullRows|' + JSON.serialize(EntryHookProbe.observe('nullRows')));
System.debug('OBSERVED|reflected|' + JSON.serialize(EntryHookProbe.observe('reflected')));
System.debug('OBSERVED|wrongElement|' + JSON.serialize(EntryHookProbe.observe('wrongElement')));
System.debug('OBSERVED|nonVoid|' + JSON.serialize(EntryHookProbe.observe('nonVoid')));
