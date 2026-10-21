trigger ActiveRowGuard on Account (before update) {
    for (Account row : Trigger.new) ActiveRowProbe.handle(row);
}
