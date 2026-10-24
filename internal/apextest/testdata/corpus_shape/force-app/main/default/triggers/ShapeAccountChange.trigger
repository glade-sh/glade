trigger ShapeAccountChange on AccountChangeEvent (after insert) { ShapeChange.accept(Trigger.new); }
