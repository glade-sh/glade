@IsTest
private class ModelsDtoObservationTest {
    @IsTest static void requestShape() {
        aiplatform.ModelsAPI.createGenerations_Request request = new aiplatform.ModelsAPI.createGenerations_Request();
        aiplatform.ModelsAPI_GenerationRequest body = new aiplatform.ModelsAPI_GenerationRequest();
        body.prompt = 'Owned request text';
        request.modelName = 'synthetic-model';
        request.body = body;
        aiplatform.ModelsAPI.createGenerations_Response response = new aiplatform.ModelsAPI.createGenerations_Response();
        aiplatform.ModelsAPI client = new aiplatform.ModelsAPI();
        System.assert(false,'OBSERVED|requestShape|'+JSON.serialize(new Map<String,Object>{'model'=>request.modelName,'prompt'=>request.body.prompt,'responseBodyNull'=>response.Code200==null,'clientPresent'=>client!=null}));
    }
}
