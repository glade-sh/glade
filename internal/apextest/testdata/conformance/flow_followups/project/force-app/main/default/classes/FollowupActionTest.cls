@IsTest private class FollowupActionTest { @IsTest static void observe() { System.assert(false, 'FF_ACTION|' + JSON.serialize(FollowupActionProbe.observe())); } }
