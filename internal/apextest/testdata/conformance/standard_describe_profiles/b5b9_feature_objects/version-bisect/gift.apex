Map<String,Schema.SObjectType> m=Schema.getGlobalDescribe();
System.debug('ROW GiftCommitment.LastNextGenCmtProcDtTm '+m.get('GiftCommitment').getDescribe().fields.getMap().containsKey('LastNextGenCmtProcDtTm'));
System.debug('ROW GiftCommitment.LastNextGenCmtProcError '+m.get('GiftCommitment').getDescribe().fields.getMap().containsKey('LastNextGenCmtProcError'));
System.debug('ROW GiftCommitment.PartyPhilanthropicRsrchPrflId '+m.get('GiftCommitment').getDescribe().fields.getMap().containsKey('PartyPhilanthropicRsrchPrflId'));
System.debug('ROW GiftTransaction.GenerationalCohort '+m.get('GiftTransaction').getDescribe().fields.getMap().containsKey('GenerationalCohort'));
System.debug('ROW GiftTransaction.PartyPhilanthropicRsrchPrflId '+m.get('GiftTransaction').getDescribe().fields.getMap().containsKey('PartyPhilanthropicRsrchPrflId'));
System.debug('ROW GiftTransaction.TaxDeductionAmount '+m.get('GiftTransaction').getDescribe().fields.getMap().containsKey('TaxDeductionAmount'));
