package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func extractC(root *sitter.Node, content []byte, file string) *ParseResult {
	return extractCLike(root, content, file)
}

func extractCpp(root *sitter.Node, content []byte, file string) *ParseResult {
	return extractCLike(root, content, file)
}

func extractCLike(root *sitter.Node, content []byte, file string) *ParseResult {
	result := &ParseResult{File: file}

	walkTree(root, func(n *sitter.Node) {
		switch n.Type() {
		case "function_definition":
			declarator := childByFieldName(n, "declarator")
			if declarator == nil {
				return
			}
			nameStr := extractCDeclaratorName(declarator, content)
			if nameStr == "" {
				return
			}
			sig := nodeText(declarator, content)
			if retType := childByFieldName(n, "type"); retType != nil {
				sig = nodeText(retType, content) + " " + sig
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:      nameStr,
				Kind:      "function",
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Exported:  true,
			})

		case "declaration":
			// Could be function declaration or variable
			declarator := childByFieldName(n, "declarator")
			if declarator == nil {
				return
			}
			// Skip function prototypes, handle vars
			if declarator.Type() == "init_declarator" || declarator.Type() == "identifier" {
				nameStr := extractCDeclaratorName(declarator, content)
				if nameStr != "" {
					result.Symbols = append(result.Symbols, Symbol{
						Name:     nameStr,
						Kind:     "var",
						File:     file,
						Line:     int(n.StartPoint().Row) + 1,
						EndLine:  int(n.EndPoint().Row) + 1,
						Exported: true,
					})
				}
			}

		case "struct_specifier":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "struct",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: true,
			})

		case "enum_specifier":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "enum",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: true,
			})

		case "type_definition":
			declarator := childByFieldName(n, "declarator")
			if declarator == nil {
				return
			}
			nameStr := extractCDeclaratorName(declarator, content)
			if nameStr == "" {
				nameStr = nodeText(declarator, content)
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "type",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: true,
			})

		case "class_specifier": // C++ only
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
				Exported: true,
			})

		case "namespace_definition": // C++ only
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nodeText(name, content),
				Kind:     "module",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: true,
			})

		case "preproc_include":
			path := childByFieldName(n, "path")
			if path == nil {
				return
			}
			pathStr := nodeText(path, content)
			pathStr = trimQuotes(pathStr)
			// Remove angle brackets
			if len(pathStr) >= 2 && pathStr[0] == '<' && pathStr[len(pathStr)-1] == '>' {
				pathStr = pathStr[1 : len(pathStr)-1]
			}
			result.Imports = append(result.Imports, Import{
				File: file,
				Path: pathStr,
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

func extractCDeclaratorName(n *sitter.Node, content []byte) string {
	switch n.Type() {
	case "identifier":
		return nodeText(n, content)
	case "function_declarator":
		declarator := childByFieldName(n, "declarator")
		if declarator != nil {
			return extractCDeclaratorName(declarator, content)
		}
	case "pointer_declarator":
		declarator := childByFieldName(n, "declarator")
		if declarator != nil {
			return extractCDeclaratorName(declarator, content)
		}
	case "init_declarator":
		declarator := childByFieldName(n, "declarator")
		if declarator != nil {
			return extractCDeclaratorName(declarator, content)
		}
	case "parenthesized_declarator":
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() != "(" && child.Type() != ")" {
				return extractCDeclaratorName(child, content)
			}
		}
	}
	return ""
}
