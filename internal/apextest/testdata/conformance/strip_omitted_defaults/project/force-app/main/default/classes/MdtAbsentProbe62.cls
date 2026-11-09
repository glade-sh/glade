@isTest class MdtAbsentProbe62 {
  static String attempt(Integer which) {
    try {
      Object value;
      switch on which {
        when 0 { value = StripProbeCfg__mdt.getInstance('absent') == null; }
        when 1 { value = StripProbeCfg__mdt.getInstance('absent').Flag__c; }
        when 2 { value = StripProbeCfg__mdt.getInstance('absent').Note__c; }
        when 3 { value = StripProbeCfg__mdt.getInstance('absent').DeveloperName; }
        when 4 { StripProbeCfg__mdt row = StripProbeCfg__mdt.getInstance('absent'); value = row.Flag__c; }
        when 5 { value = StripProbeCfg__mdt.getAll().get('absent').Flag__c; }
        when 6 { value = StripProbeCfg__mdt.getInstance('absent')?.Flag__c; }
        when 7 { value = StripProbeCfg__mdt.getInstance('Present').Flag__c; }
        when 8 { value = StripProbeCfg__mdt.getInstance('Present').Note__c; }
        when else { Boolean flag = StripProbeCfg__mdt.getInstance('absent').Flag__c; value = flag; }
      }
      return 'value=' + value;
    } catch (Exception e) {
      return 'threw=' + e.getTypeName();
    }
  }
  @isTest static void matrix() {
    String[] names = new String[]{'isNull','flag','note','developerName','localFlag','getAllFlag','safeNavFlag','presentFlag','presentNote','assignFlag'};
    String out = '';
    for (Integer i = 0; i < names.size(); i++) out += names[i] + ' ' + attempt(i) + '\n';
    System.assert(false, '\n' + out);
  }
}
