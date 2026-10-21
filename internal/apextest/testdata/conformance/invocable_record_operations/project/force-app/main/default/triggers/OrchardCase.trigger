trigger OrchardCase on Case (after update) { OrchardObservationTest.updates++; }
