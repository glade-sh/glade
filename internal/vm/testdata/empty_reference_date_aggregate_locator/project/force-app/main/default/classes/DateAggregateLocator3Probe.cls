// Intentional failure transports a native observation.
@IsTest private class DateAggregateLocator3Probe { @IsTest static void observe() { System.assert(false,'DATE_AGGREGATE_OBSERVATION|count-date|'+JSON.serialize(DateAggregateLocatorObservation.capture('count-date'))); } }
