public class SummaryValue {
 private List<Opportunity> rows;
 public SummaryValue(List<Opportunity> rows) {this.rows=rows;}
 public Decimal average() {Decimal sum=0;for(Opportunity row:rows)sum+=row.Amount;return sum/rows.size();}
 public Integer count() {return rows.size();}
 public Decimal last() {return rows[rows.size()-1].Amount;}
}
