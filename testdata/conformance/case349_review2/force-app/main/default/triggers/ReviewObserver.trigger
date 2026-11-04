trigger ReviewObserver on Account (before insert, before update) { ReviewTrace.hits++; }
