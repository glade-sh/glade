Id submitted = System.enqueueJob(new ReturnIdentityQueue());
Object boxed = submitted;
String assigned = submitted;
String castText = (String) submitted;
Map<String,Object> observation = new Map<String,Object>{
    'typedValueOfLength' => String.valueOf(submitted).length(),
    'boxedValueOfLength' => String.valueOf(boxed).length(),
    'typedConcatLength' => ('' + submitted).length(),
    'castIdValueOfLength' => String.valueOf((Id) submitted).length(),
    'castIdConcatLength' => ('' + (Id) submitted).length(),
    'typedAssignedLength' => assigned.length(),
    'typedCastLength' => castText.length(),
    'typedJsonLength' => JSON.serialize(submitted).length(),
    'stringArgumentLength' => AsyncReturnIdentityProbe.stringLength(System.enqueueJob(new ReturnIdentityQueue())),
    'stringReturnLength' => AsyncReturnIdentityProbe.stringReturn().length(),
    'idReturnValueOfLength' => String.valueOf(AsyncReturnIdentityProbe.idReturn()).length(),
    'objectReturnValueOfLength' => String.valueOf(AsyncReturnIdentityProbe.objectReturn()).length()
};
System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(observation));
