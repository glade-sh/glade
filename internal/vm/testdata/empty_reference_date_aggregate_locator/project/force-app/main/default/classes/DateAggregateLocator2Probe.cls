// Intentional failure transports a native observation.
@IsTest private class DateAggregateLocator2Probe { @IsTest static void observe() { System.assert(false,'DATE_AGGREGATE_OBSERVATION|maximum-date-or-decimal|'+JSON.serialize(DateAggregateLocatorObservation.capture('maximum-date-or-decimal'))); } }
