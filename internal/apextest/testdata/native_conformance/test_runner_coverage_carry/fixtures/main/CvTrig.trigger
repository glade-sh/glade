trigger CvTrig on Account (before insert) {
    for (Account a : Trigger.new) {
        if (a.Name == 'cv-trig') {
            a.Description = 'hit';
        } else {
            a.Description = 'miss';
        }
    }
}
