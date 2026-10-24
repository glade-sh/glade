public class LoopShapes {
    @AuraEnabled(cacheable=true)
    public static String run() {
String text='abcd'.repeat(256); Integer chars=0; for(Integer i=0;i<100;i++){chars+=text.mid(i,1).length()+text.left(1).length()+text.right(1).length();} System.assertEquals(300,chars);
 Dom.Document doc=new Dom.Document(); doc.load('<root>'+('<item key="v">abc</item>').repeat(100)+'</root>'); System.assertEquals(100,doc.getRootElement().getChildElements().size()); System.assertEquals('abc',doc.getRootElement().getChildElements().get(99).getText());
 XmlStreamReader reader=new XmlStreamReader('<root>Ω<x/>tail</root>'); Integer events=0; String locations=''; while(reader.hasNext()){reader.next();events++;locations+=reader.getLocation()+';';} System.assertEquals(7,events); System.assertEquals('Line: 1 Column: 7;Line: 1 Column: 9;Line: 1 Column: 12;Line: 1 Column: 12;Line: 1 Column: 17;Line: 1 Column: 23;;',locations);
 List<Integer> pixels=new List<Integer>{1,2,3,4}; for(Integer i=0;i<4000;i++){Integer at=i&3; pixels.set(at,(pixels.get(at)+i)&255);} System.assertEquals(177,pixels.get(0)); System.assertEquals(154,pixels.get(1)); System.assertEquals(131,pixels.get(2)); System.assertEquals(108,pixels.get(3));
        return chars+':'+doc.getRootElement().getChildElements().size()+':'+events+':'+pixels.get(0)+','+pixels.get(1)+','+pixels.get(2)+','+pixels.get(3)+':'+locations;
    }
    public String getOutcome() { return run(); }
}
