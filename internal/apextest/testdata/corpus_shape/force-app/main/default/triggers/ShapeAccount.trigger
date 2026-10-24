trigger ShapeAccount on Account (before insert) { ShapeDispatcher.dispatch(Trigger.operationType, Trigger.new); }
