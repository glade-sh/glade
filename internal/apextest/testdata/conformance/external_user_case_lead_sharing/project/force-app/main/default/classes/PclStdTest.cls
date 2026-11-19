@isTest private class PclStdTest {
    @isTest static void run() { PclSupport.setup(); User u = PclSupport.newUser('pclstd', PclSupport.profileId('Standard User'), null, null); insert u; PclSupport.observe('std', u); }
}
