// Intentional failure transports a native observation.
@IsTest private class DateAggregateLocator1Probe { @IsTest static void observe() { System.assert(false,'DATE_AGGREGATE_OBSERVATION|maximum-date|'+JSON.serialize(DateAggregateLocatorObservation.capture('maximum-date'))); } }
