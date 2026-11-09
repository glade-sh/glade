@IsTest private class FormulaBrProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(FormulaBrProbe.observe()));
    }
}
