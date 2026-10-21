@IsTest private class TransactionStaticTest {
@IsTest static void direct_immediate() {System.assert(false,'OBSERVED|direct_immediate|'+JSON.serialize(TransactionStaticState.observe('direct',false)));}
@IsTest static void direct_committed() {System.assert(false,'OBSERVED|direct_committed|'+JSON.serialize(TransactionStaticState.observe('direct',true)));}
@IsTest static void queue_immediate() {System.assert(false,'OBSERVED|queue_immediate|'+JSON.serialize(TransactionStaticState.observe('queue',false)));}
@IsTest static void queue_committed() {System.assert(false,'OBSERVED|queue_committed|'+JSON.serialize(TransactionStaticState.observe('queue',true)));}
@IsTest static void future_immediate() {System.assert(false,'OBSERVED|future_immediate|'+JSON.serialize(TransactionStaticState.observe('future',false)));}
@IsTest static void future_committed() {System.assert(false,'OBSERVED|future_committed|'+JSON.serialize(TransactionStaticState.observe('future',true)));}
@IsTest static void batch_immediate() {System.assert(false,'OBSERVED|batch_immediate|'+JSON.serialize(TransactionStaticState.observe('batch',false)));}
@IsTest static void batch_committed() {System.assert(false,'OBSERVED|batch_committed|'+JSON.serialize(TransactionStaticState.observe('batch',true)));}
}
