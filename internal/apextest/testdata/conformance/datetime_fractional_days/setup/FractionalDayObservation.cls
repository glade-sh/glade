public class FractionalDayObservation {
 public static List<Datetime> observe() {
  Datetime base=Datetime.newInstanceGmt(2020,1,2,0,0,0);
  Decimal fraction=0.5; Double doubleFraction=0.5; Integer whole=1;
  return new List<Datetime>{base-0.5,base+0.5,base-fraction,base+fraction,
    base-doubleFraction,base+doubleFraction,base-whole,base+whole};
 }
}
