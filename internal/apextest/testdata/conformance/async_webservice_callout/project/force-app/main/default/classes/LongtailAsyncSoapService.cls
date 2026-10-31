public class LongtailAsyncSoapService {
    public class EchoRequest {
        public String value;
        private String[] value_type_info = new String[]{'value', 'urn:glade:echo', 'xsd:string', '0', '1', 'false'};
        private String[] apex_schema_type_info = new String[]{'urn:glade:echo', 'true', 'false'};
        private String[] field_order_type_info = new String[]{'value'};
    }

    public class EchoResponse {
        public String value;
        private String[] value_type_info = new String[]{'value', 'urn:glade:echo', 'xsd:string', '0', '1', 'false'};
        private String[] apex_schema_type_info = new String[]{'urn:glade:echo', 'true', 'false'};
        private String[] field_order_type_info = new String[]{'value'};
    }

    public class EchoFuture extends System.WebServiceCalloutFuture {
        public String getValue() {
            EchoResponse response = (EchoResponse)System.WebServiceCallout.endInvoke(this);
            return response.value;
        }
    }

    public String endpoint = 'https://async-soap.invalid/endpoint';

    public EchoFuture beginEcho(System.Continuation continuation, String value) {
        EchoRequest request = new EchoRequest();
        request.value = value;
        return (EchoFuture)System.WebServiceCallout.beginInvoke(
            this,
            request,
            EchoFuture.class,
            continuation,
            new String[]{endpoint, 'urn:glade:echo', 'urn:glade:echo', 'Echo', 'urn:glade:echo', 'EchoResponse', 'LongtailAsyncSoapService.EchoResponse'}
        );
    }
}
