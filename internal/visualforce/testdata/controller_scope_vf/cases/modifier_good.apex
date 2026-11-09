public class CaseController { public String getObserved() { FINAL Integer x = 7; Transient Integer y = 2; System.assertEquals(9,x+y); return 'VF_NATIVE_OK'; } }
