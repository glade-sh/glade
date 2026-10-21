public class CalendarTokenProbe {
 public static String sample() {
  Map<String,String> rows = new Map<String,String>();
  Map<String,String> tokens = new Map<String,String>{
   'padded' => '2024-09-07',
   'unpadded' => '2024-9-7',
   'oneDigitYear' => '8-09-07',
   'twoDigitYear' => '18-09-07',
   'isoUtc' => '2024-09-07T13:14:15Z',
   'isoOffset' => '2024-09-07T13:14:15+02:00',
   'localClock' => '2024-09-07 13:14:15',
   'invalid' => 'not-a-date'
  };
  for (String name : tokens.keySet()) {
   try {
    Date value = (Date)JSON.deserialize(JSON.serialize(tokens.get(name)), Date.class);
    rows.put(name, 'value:' + JSON.serialize(value));
   } catch (Exception problem) {
    rows.put(name, 'error:' + problem.getTypeName());
   }
  }
  return JSON.serialize(rows);
 }
}
