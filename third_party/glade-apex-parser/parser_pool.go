package apexast

import "sync"

var parserPool = sync.Pool{New: func() any { return NewParser() }}

// ParseSource parses source using a parser exclusively borrowed from a pool.
func ParseSource(path, source string) File {
	p := parserPool.Get().(*Parser)
	defer parserPool.Put(p)
	return p.ParseSource(path, source)
}

// ParseSourceAST parses source using a parser exclusively borrowed from a pool.
func ParseSourceAST(path, source string) ASTFile {
	p := parserPool.Get().(*Parser)
	defer parserPool.Put(p)
	return p.ParseSourceAST(path, source)
}
