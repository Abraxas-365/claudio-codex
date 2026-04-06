package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func extractJS(root *sitter.Node, content []byte, file string) *ParseResult {
	return extractJSLike(root, content, file)
}

func extractTS(root *sitter.Node, content []byte, file string) *ParseResult {
	return extractJSLike(root, content, file)
}

func extractJSLike(root *sitter.Node, content []byte, file string) *ParseResult {
	result := &ParseResult{File: file}

	walkTree(root, func(n *sitter.Node) {
		switch n.Type() {
		case "function_declaration", "generator_function_declaration":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			sig := nameStr
			if params := childByFieldName(n, "parameters"); params != nil {
				sig += nodeText(params, content)
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:      nameStr,
				Kind:      "function",
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Exported:  jsIsExported(n),
			})

		case "class_declaration":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "class",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: jsIsExported(n),
			})

		case "method_definition":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			parent := findJSClass(n, content)
			sig := nameStr
			if params := childByFieldName(n, "parameters"); params != nil {
				sig += nodeText(params, content)
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:      nameStr,
				Kind:      "method",
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Parent:    parent,
				Exported:  true,
			})

		case "interface_declaration":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "interface",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: jsIsExported(n),
			})

		case "type_alias_declaration":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "type",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: jsIsExported(n),
			})

		case "lexical_declaration", "variable_declaration":
			// const foo = ..., let bar = ..., var baz = ...
			extractJSVarDecl(n, content, file, result)

		case "import_statement":
			src := childByFieldName(n, "source")
			if src == nil {
				return
			}
			path := trimQuotes(nodeText(src, content))
			result.Imports = append(result.Imports, Import{
				File: file,
				Path: path,
			})

		case "call_expression":
			fn := childByFieldName(n, "function")
			if fn == nil {
				return
			}
			target := nodeText(fn, content)
			caller := findEnclosingFunction(n, content)
			result.Refs = append(result.Refs, Ref{
				Caller: caller,
				Target: target,
				File:   file,
				Line:   int(n.StartPoint().Row) + 1,
			})
		}
	})

	return result
}

func extractJSVarDecl(n *sitter.Node, content []byte, file string, result *ParseResult) {
	for i := 0; i < int(n.ChildCount()); i++ {
		decl := n.Child(i)
		if decl.Type() != "variable_declarator" {
			continue
		}
		name := childByFieldName(decl, "name")
		if name == nil {
			continue
		}
		nameStr := nodeText(name, content)

		// Check if value is an arrow function or function expression
		kind := "var"
		value := childByFieldName(decl, "value")
		if value != nil {
			switch value.Type() {
			case "arrow_function", "function":
				kind = "function"
			}
		}

		result.Symbols = append(result.Symbols, Symbol{
			Name:     nameStr,
			Kind:     kind,
			File:     file,
			Line:     int(decl.StartPoint().Row) + 1,
			EndLine:  int(decl.EndPoint().Row) + 1,
			Exported: jsIsExported(n),
		})
	}
}

func jsIsExported(n *sitter.Node) bool {
	parent := n.Parent()
	if parent == nil {
		return false
	}
	return parent.Type() == "export_statement"
}

func findJSClass(n *sitter.Node, content []byte) string {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Type() == "class_declaration" || p.Type() == "class" {
			if name := childByFieldName(p, "name"); name != nil {
				return nodeText(name, content)
			}
		}
	}
	return ""
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '`' && s[len(s)-1] == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
