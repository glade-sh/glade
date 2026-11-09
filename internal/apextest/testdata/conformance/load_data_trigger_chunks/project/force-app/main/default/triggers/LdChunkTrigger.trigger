trigger LdChunkTrigger on Account (before insert) { for (Account a : Trigger.new) { if (a.Name != null && a.Name.startsWith('LdChunk')) { a.AccountNumber = 'trg'; } } }
