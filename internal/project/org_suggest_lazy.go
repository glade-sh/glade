package project

import (
	"regexp"
	"sync"
)

type lazySetupRegexp struct {
	once     sync.Once
	pattern  string
	compiled *regexp.Regexp
}

func newSetupRegexp(pattern string) *lazySetupRegexp {
	return &lazySetupRegexp{pattern: pattern}
}

func (r *lazySetupRegexp) get() *regexp.Regexp {
	r.once.Do(func() { r.compiled = regexp.MustCompile(r.pattern) })
	return r.compiled
}
