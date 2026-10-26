@IsTest private class PublishingDateArgumentTest {
    @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(PublishingDateArgument.observe())); }
}
