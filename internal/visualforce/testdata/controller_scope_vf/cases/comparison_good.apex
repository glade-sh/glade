public class CaseController { public String getObserved() { Integer a=1,b=2,c=3,d=2; Boolean x = a < b && c > d; System.assertEquals(true,x); return 'VF_NATIVE_OK'; } }
