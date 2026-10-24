public class RateWindow {
 private Date firstDay, nextDay;
 private Decimal rate;
 public RateWindow(Date firstDay,Date nextDay,Decimal rate) {this.firstDay=firstDay;this.nextDay=nextDay;this.rate=rate;}
 public Boolean includes(Datetime day) { return day!=null && nextDay>day && day>=firstDay; }
 public Decimal convert(Datetime day,Decimal value) { return includes(day) ? rate*value : value; }
}
