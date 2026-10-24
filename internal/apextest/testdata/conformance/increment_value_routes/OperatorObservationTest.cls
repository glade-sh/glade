@IsTest private class OperatorObservationTest {
@IsTest static void local_integer() {
Integer value = 2;
List<String> observations = new List<String>();
Integer result0 = value++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(value));
Integer result1 = ++value; observations.add(String.valueOf(result1));
observations.add(String.valueOf(value));
Integer result2 = value--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(value));
Integer result3 = --value; observations.add(String.valueOf(result3));
observations.add(String.valueOf(value));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void field_integer() {
OperatorCell cell = new OperatorCell();
List<String> observations = new List<String>();
Integer result0 = cell.integerField++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(cell.integerField));
Integer result1 = ++cell.integerField; observations.add(String.valueOf(result1));
observations.add(String.valueOf(cell.integerField));
Integer result2 = cell.integerField--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(cell.integerField));
Integer result3 = --cell.integerField; observations.add(String.valueOf(result3));
observations.add(String.valueOf(cell.integerField));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void property_integer() {
OperatorCell cell = new OperatorCell();
List<String> observations = new List<String>();
Integer result0 = cell.integerProperty++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(cell.integerProperty));
Integer result1 = ++cell.integerProperty; observations.add(String.valueOf(result1));
observations.add(String.valueOf(cell.integerProperty));
Integer result2 = cell.integerProperty--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(cell.integerProperty));
Integer result3 = --cell.integerProperty; observations.add(String.valueOf(result3));
observations.add(String.valueOf(cell.integerProperty));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void list_integer() {
List<Integer> values = new List<Integer>{2};
List<String> observations = new List<String>();
values[0]++;
observations.add(String.valueOf(values[0]));
++values[0];
observations.add(String.valueOf(values[0]));
values[0]--;
observations.add(String.valueOf(values[0]));
--values[0];
observations.add(String.valueOf(values[0]));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void array_integer() {
Integer[] values = new Integer[]{2};
List<String> observations = new List<String>();
values[0]++;
observations.add(String.valueOf(values[0]));
++values[0];
observations.add(String.valueOf(values[0]));
values[0]--;
observations.add(String.valueOf(values[0]));
--values[0];
observations.add(String.valueOf(values[0]));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void map_field_integer() {
Map<String,OperatorCell> cells = new Map<String,OperatorCell>{'one'=>new OperatorCell()};
List<String> observations = new List<String>();
cells.get('one').integerField++;
observations.add(String.valueOf(cells.get('one').integerField));
++cells.get('one').integerField;
observations.add(String.valueOf(cells.get('one').integerField));
cells.get('one').integerField--;
observations.add(String.valueOf(cells.get('one').integerField));
--cells.get('one').integerField;
observations.add(String.valueOf(cells.get('one').integerField));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void boxed_integer() {
Object boxed = 2; Integer value = (Integer)boxed;
List<String> observations = new List<String>();
Integer result0 = value++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(value));
Integer result1 = ++value; observations.add(String.valueOf(result1));
observations.add(String.valueOf(value));
Integer result2 = value--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(value));
Integer result3 = --value; observations.add(String.valueOf(result3));
observations.add(String.valueOf(value));
observations.add(String.valueOf(boxed));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void local_long() {
Long value = 2L;
List<String> observations = new List<String>();
Long result0 = value++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(value));
Long result1 = ++value; observations.add(String.valueOf(result1));
observations.add(String.valueOf(value));
Long result2 = value--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(value));
Long result3 = --value; observations.add(String.valueOf(result3));
observations.add(String.valueOf(value));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void field_long() {
OperatorCell cell = new OperatorCell();
List<String> observations = new List<String>();
Long result0 = cell.longField++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(cell.longField));
Long result1 = ++cell.longField; observations.add(String.valueOf(result1));
observations.add(String.valueOf(cell.longField));
Long result2 = cell.longField--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(cell.longField));
Long result3 = --cell.longField; observations.add(String.valueOf(result3));
observations.add(String.valueOf(cell.longField));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void property_long() {
OperatorCell cell = new OperatorCell();
List<String> observations = new List<String>();
Long result0 = cell.longProperty++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(cell.longProperty));
Long result1 = ++cell.longProperty; observations.add(String.valueOf(result1));
observations.add(String.valueOf(cell.longProperty));
Long result2 = cell.longProperty--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(cell.longProperty));
Long result3 = --cell.longProperty; observations.add(String.valueOf(result3));
observations.add(String.valueOf(cell.longProperty));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void list_long() {
List<Long> values = new List<Long>{2L};
List<String> observations = new List<String>();
values[0]++;
observations.add(String.valueOf(values[0]));
++values[0];
observations.add(String.valueOf(values[0]));
values[0]--;
observations.add(String.valueOf(values[0]));
--values[0];
observations.add(String.valueOf(values[0]));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void array_long() {
Long[] values = new Long[]{2L};
List<String> observations = new List<String>();
values[0]++;
observations.add(String.valueOf(values[0]));
++values[0];
observations.add(String.valueOf(values[0]));
values[0]--;
observations.add(String.valueOf(values[0]));
--values[0];
observations.add(String.valueOf(values[0]));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void map_field_long() {
Map<String,OperatorCell> cells = new Map<String,OperatorCell>{'one'=>new OperatorCell()};
List<String> observations = new List<String>();
cells.get('one').longField++;
observations.add(String.valueOf(cells.get('one').longField));
++cells.get('one').longField;
observations.add(String.valueOf(cells.get('one').longField));
cells.get('one').longField--;
observations.add(String.valueOf(cells.get('one').longField));
--cells.get('one').longField;
observations.add(String.valueOf(cells.get('one').longField));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void boxed_long() {
Object boxed = 2L; Long value = (Long)boxed;
List<String> observations = new List<String>();
Long result0 = value++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(value));
Long result1 = ++value; observations.add(String.valueOf(result1));
observations.add(String.valueOf(value));
Long result2 = value--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(value));
Long result3 = --value; observations.add(String.valueOf(result3));
observations.add(String.valueOf(value));
observations.add(String.valueOf(boxed));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void local_decimal() {
Decimal value = 2.0;
List<String> observations = new List<String>();
Decimal result0 = value++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(value));
Decimal result1 = ++value; observations.add(String.valueOf(result1));
observations.add(String.valueOf(value));
Decimal result2 = value--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(value));
Decimal result3 = --value; observations.add(String.valueOf(result3));
observations.add(String.valueOf(value));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void field_decimal() {
OperatorCell cell = new OperatorCell();
List<String> observations = new List<String>();
Decimal result0 = cell.decimalField++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(cell.decimalField));
Decimal result1 = ++cell.decimalField; observations.add(String.valueOf(result1));
observations.add(String.valueOf(cell.decimalField));
Decimal result2 = cell.decimalField--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(cell.decimalField));
Decimal result3 = --cell.decimalField; observations.add(String.valueOf(result3));
observations.add(String.valueOf(cell.decimalField));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void property_decimal() {
OperatorCell cell = new OperatorCell();
List<String> observations = new List<String>();
Decimal result0 = cell.decimalProperty++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(cell.decimalProperty));
Decimal result1 = ++cell.decimalProperty; observations.add(String.valueOf(result1));
observations.add(String.valueOf(cell.decimalProperty));
Decimal result2 = cell.decimalProperty--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(cell.decimalProperty));
Decimal result3 = --cell.decimalProperty; observations.add(String.valueOf(result3));
observations.add(String.valueOf(cell.decimalProperty));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void list_decimal() {
List<Decimal> values = new List<Decimal>{2.0};
List<String> observations = new List<String>();
values[0]++;
observations.add(String.valueOf(values[0]));
++values[0];
observations.add(String.valueOf(values[0]));
values[0]--;
observations.add(String.valueOf(values[0]));
--values[0];
observations.add(String.valueOf(values[0]));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void array_decimal() {
Decimal[] values = new Decimal[]{2.0};
List<String> observations = new List<String>();
values[0]++;
observations.add(String.valueOf(values[0]));
++values[0];
observations.add(String.valueOf(values[0]));
values[0]--;
observations.add(String.valueOf(values[0]));
--values[0];
observations.add(String.valueOf(values[0]));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void map_field_decimal() {
Map<String,OperatorCell> cells = new Map<String,OperatorCell>{'one'=>new OperatorCell()};
List<String> observations = new List<String>();
cells.get('one').decimalField++;
observations.add(String.valueOf(cells.get('one').decimalField));
++cells.get('one').decimalField;
observations.add(String.valueOf(cells.get('one').decimalField));
cells.get('one').decimalField--;
observations.add(String.valueOf(cells.get('one').decimalField));
--cells.get('one').decimalField;
observations.add(String.valueOf(cells.get('one').decimalField));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void boxed_decimal() {
Object boxed = 2.0; Decimal value = (Decimal)boxed;
List<String> observations = new List<String>();
Decimal result0 = value++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(value));
Decimal result1 = ++value; observations.add(String.valueOf(result1));
observations.add(String.valueOf(value));
Decimal result2 = value--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(value));
Decimal result3 = --value; observations.add(String.valueOf(result3));
observations.add(String.valueOf(value));
observations.add(String.valueOf(boxed));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void local_double() {
Double value = 2.0D;
List<String> observations = new List<String>();
Double result0 = value++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(value));
Double result1 = ++value; observations.add(String.valueOf(result1));
observations.add(String.valueOf(value));
Double result2 = value--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(value));
Double result3 = --value; observations.add(String.valueOf(result3));
observations.add(String.valueOf(value));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void field_double() {
OperatorCell cell = new OperatorCell();
List<String> observations = new List<String>();
Double result0 = cell.doubleField++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(cell.doubleField));
Double result1 = ++cell.doubleField; observations.add(String.valueOf(result1));
observations.add(String.valueOf(cell.doubleField));
Double result2 = cell.doubleField--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(cell.doubleField));
Double result3 = --cell.doubleField; observations.add(String.valueOf(result3));
observations.add(String.valueOf(cell.doubleField));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void property_double() {
OperatorCell cell = new OperatorCell();
List<String> observations = new List<String>();
Double result0 = cell.doubleProperty++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(cell.doubleProperty));
Double result1 = ++cell.doubleProperty; observations.add(String.valueOf(result1));
observations.add(String.valueOf(cell.doubleProperty));
Double result2 = cell.doubleProperty--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(cell.doubleProperty));
Double result3 = --cell.doubleProperty; observations.add(String.valueOf(result3));
observations.add(String.valueOf(cell.doubleProperty));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void list_double() {
List<Double> values = new List<Double>{2.0D};
List<String> observations = new List<String>();
values[0]++;
observations.add(String.valueOf(values[0]));
++values[0];
observations.add(String.valueOf(values[0]));
values[0]--;
observations.add(String.valueOf(values[0]));
--values[0];
observations.add(String.valueOf(values[0]));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void array_double() {
Double[] values = new Double[]{2.0D};
List<String> observations = new List<String>();
values[0]++;
observations.add(String.valueOf(values[0]));
++values[0];
observations.add(String.valueOf(values[0]));
values[0]--;
observations.add(String.valueOf(values[0]));
--values[0];
observations.add(String.valueOf(values[0]));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void map_field_double() {
Map<String,OperatorCell> cells = new Map<String,OperatorCell>{'one'=>new OperatorCell()};
List<String> observations = new List<String>();
cells.get('one').doubleField++;
observations.add(String.valueOf(cells.get('one').doubleField));
++cells.get('one').doubleField;
observations.add(String.valueOf(cells.get('one').doubleField));
cells.get('one').doubleField--;
observations.add(String.valueOf(cells.get('one').doubleField));
--cells.get('one').doubleField;
observations.add(String.valueOf(cells.get('one').doubleField));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void boxed_double() {
Object boxed = 2.0D; Double value = (Double)boxed;
List<String> observations = new List<String>();
Double result0 = value++; observations.add(String.valueOf(result0));
observations.add(String.valueOf(value));
Double result1 = ++value; observations.add(String.valueOf(result1));
observations.add(String.valueOf(value));
Double result2 = value--; observations.add(String.valueOf(result2));
observations.add(String.valueOf(value));
Double result3 = --value; observations.add(String.valueOf(result3));
observations.add(String.valueOf(value));
observations.add(String.valueOf(boxed));

System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void null_local() {
List<String> observations = new List<String>();
try { Integer value; value++; } catch (Exception ex) { observations.add(ex.getTypeName()+'|'+ex.getMessage()); }
System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void null_field() {
List<String> observations = new List<String>();
try { OperatorCell cell = new OperatorCell(); cell.integerField=null; cell.integerField++; } catch (Exception ex) { observations.add(ex.getTypeName()+'|'+ex.getMessage()); }
System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void null_list() {
List<String> observations = new List<String>();
try { List<Integer> values = new List<Integer>{null}; values[0]++; } catch (Exception ex) { observations.add(ex.getTypeName()+'|'+ex.getMessage()); }
System.assert(false,'OBS|'+JSON.serialize(observations));
}
@IsTest static void integer_overflow() {
List<String> observations = new List<String>();
try { Integer value=2147483647; value++; observations.add(String.valueOf(value)); value--; observations.add(String.valueOf(value)); } catch (Exception ex) { observations.add(ex.getTypeName()+'|'+ex.getMessage()); }
System.assert(false,'OBS|'+JSON.serialize(observations));
}
}
