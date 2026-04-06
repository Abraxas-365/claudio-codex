package parser

import (
	"context"
	"fmt"
	"path/filepath"

	sitter "github.com/smacker/go-tree-sitter"
)

// Parse parses a source file and returns extracted symbols, refs, and imports.
func Parse(path string, content []byte) (*ParseResult, error) {
	ext := filepath.Ext(path)
	entry, ok := languages[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}

	p := sitter.NewParser()
	p.SetLanguage(entry.lang)

	tree, err := p.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	defer tree.Close()

	root := tree.RootNode()
	result := entry.extract(root, content, path)
	if result == nil {
		result = &ParseResult{File: path}
	}
	result.File = path
	return result, nil
}

// nodeText returns the text of a tree-sitter node.
func nodeText(n *sitter.Node, content []byte) string {
	return n.Content(content)
}

// childByFieldName returns the child node with the given field name, or nil.
func childByFieldName(n *sitter.Node, name string) *sitter.Node {
	return n.ChildByFieldName(name)
}

// isExported checks if a name is exported (starts with uppercase).
// This is Go-specific but reused as a reasonable default for other languages.
func isExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

// walkChildren calls fn for each child node.
func walkChildren(n *sitter.Node, fn func(child *sitter.Node)) {
	for i := 0; i < int(n.ChildCount()); i++ {
		fn(n.Child(i))
	}
}

// walkTree walks the tree depth-first, calling fn for each node.
func walkTree(n *sitter.Node, fn func(node *sitter.Node)) {
	fn(n)
	for i := 0; i < int(n.ChildCount()); i++ {
		walkTree(n.Child(i), fn)
	}
}

// findEnclosingFunction finds the nearest enclosing function/method name for a node.
func findEnclosingFunction(n *sitter.Node, content []byte) string {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Type() {
		case "function_declaration", "method_declaration", "function_definition",
			"function_item", "method_definition", "function",
			"arrow_function", "generator_function_declaration":
			if name := childByFieldName(p, "name"); name != nil {
				return nodeText(name, content)
			}
		}
	}
	return ""
}
