@IsTest
private class NoteCollectorTest {
    @IsTest static void aliasBeforeHelperAppend() { observe('aliasBeforeHelperAppend', true, true); }
    @IsTest static void aliasBeforeDirectAppend() { observe('aliasBeforeDirectAppend', true, false); }
    @IsTest static void aliasAfterHelperAppend() { observe('aliasAfterHelperAppend', false, true); }
    @IsTest static void aliasAfterDirectAppend() { observe('aliasAfterDirectAppend', false, false); }
    private static void observe(String label, Boolean assignFirst, Boolean useHelper) {
        NoteCollector.Packet packet = NoteCollector.collect(assignFirst, useHelper);
        Map<String, Object> result = new Map<String, Object>{
            'ownerSize' => packet.observedSize,
            'aliasSize' => packet.notes.size(),
            'text' => packet.notes.isEmpty() ? null : packet.notes[0].text
        };
        System.assert(false, 'OBSERVED|' + label + '|' + JSON.serialize(result));
    }
}