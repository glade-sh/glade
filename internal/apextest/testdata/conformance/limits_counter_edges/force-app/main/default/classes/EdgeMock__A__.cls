@IsTest
global class EdgeMock__A__ implements HttpCalloutMock {
    global HttpResponse respond(HttpRequest req) {
        HttpResponse res = new HttpResponse();
        res.setStatusCode(200);
        res.setBody('ok');
        return res;
    }
}
