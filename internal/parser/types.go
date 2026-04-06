package parser

// Symbol represents a named entity extracted from source code.
type Symbol struct {
	Name      string
	Kind      string // function, method, type, struct, interface, class, var, const, enum, trait, module
	File      string
	Line      int
	EndLine   int
	Signature string
	Parent    string // enclosing type/class (for methods)
	Exported  bool
}

// Ref represents a reference from one symbol to another.
type Ref struct {
	Caller string // enclosing function/method name
	Target string // called function/method name
	File   string
	Line   int
}

// Import represents an import statement.
type Import struct {
	File  string
	Path  string // import path / module name
	Alias string // local alias if any
}

// ParseResult holds all extracted data from a single file.
type ParseResult struct {
	File    string
	Symbols []Symbol
	Refs    []Ref
	Imports []Import
}
