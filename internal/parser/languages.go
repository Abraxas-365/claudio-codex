package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type langEntry struct {
	lang    *sitter.Language
	extract func(n *sitter.Node, content []byte, file string) *ParseResult
}

var languages = map[string]langEntry{
	".go":   {lang: golang.GetLanguage(), extract: extractGo},
	".py":   {lang: python.GetLanguage(), extract: extractPython},
	".js":   {lang: javascript.GetLanguage(), extract: extractJS},
	".jsx":  {lang: javascript.GetLanguage(), extract: extractJS},
	".ts":   {lang: typescript.GetLanguage(), extract: extractTS},
	".tsx":  {lang: typescript.GetLanguage(), extract: extractTS},
	".rs":   {lang: rust.GetLanguage(), extract: extractRust},
	".java": {lang: java.GetLanguage(), extract: extractJava},
	".c":    {lang: c.GetLanguage(), extract: extractC},
	".h":    {lang: c.GetLanguage(), extract: extractC},
	".cpp":  {lang: cpp.GetLanguage(), extract: extractCpp},
	".cc":   {lang: cpp.GetLanguage(), extract: extractCpp},
	".hpp":  {lang: cpp.GetLanguage(), extract: extractCpp},
	".rb":   {lang: ruby.GetLanguage(), extract: extractRuby},
}

// SupportedExt returns true if the file extension is supported.
func SupportedExt(ext string) bool {
	_, ok := languages[ext]
	return ok
}
