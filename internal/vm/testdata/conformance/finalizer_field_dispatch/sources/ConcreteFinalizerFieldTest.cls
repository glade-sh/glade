@IsTest private class ConcreteFinalizerFieldTest {
    @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(ConcreteFinalizerField.observe())); }
}
