trigger CasePhaseProbe on Case (after insert) { CasePhaseCapture.observe(Trigger.new); }
