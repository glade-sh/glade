System.debug('OBSERVED|loopBelow|'+JSON.serialize(BoundaryProbe.loopBelow()));
System.debug('OBSERVED|loopAbove|'+JSON.serialize(BoundaryProbe.loopAbove()));
System.debug('OBSERVED|timeAdvance|'+JSON.serialize(BoundaryProbe.timeAdvance()));
try {
    BoundaryProbe.dateLiteral();
    System.debug('OBSERVED|dateRejected|'+JSON.serialize(new Map<String,Object>{'rejected'=>false}));
} catch (Exception problem) {
    System.debug('OBSERVED|dateRejected|'+JSON.serialize(new Map<String,Object>{'rejected'=>true,'type'=>problem.getTypeName(),'message'=>problem.getMessage()}));
}
