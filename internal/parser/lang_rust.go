package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func extractRust(root *sitter.Node, content []byte, file string) *ParseResult {
	result := &ParseResult{File: file}

	walkTree(root, func(n *sitter.Node) {
		switch n.Type() {
		case "function_item":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			sig := nameStr
			if params := childByFieldName(n, "parameters"); params != nil {
				sig += nodeText(params, content)
			}
			if ret := childByFieldName(n, "return_type"); ret != nil {
				sig += " -> " + nodeText(ret, content)
			}
			parent := findRustImpl(n, content)
			kind := "function"
			if parent != "" {
				kind = "method"
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:      nameStr,
				Kind:      kind,
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Parent:    parent,
				Exported:  rustIsPublic(n),
			})

		case "struct_item":
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
				Exported: rustIsPublic(n),
			})

		case "enum_item":
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
				Exported: rustIsPublic(n),
			})

		case "trait_item":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "trait",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: rustIsPublic(n),
			})

		case "type_item":
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
				Exported: rustIsPublic(n),
			})

		case "const_item":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nodeText(name, content),
				Kind:     "const",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: rustIsPublic(n),
			})

		case "static_item":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nodeText(name, content),
				Kind:     "var",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: rustIsPublic(n),
			})

		case "use_declaration":
			// use foo::bar::baz;
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "use_as_clause" || child.Type() == "scoped_identifier" || child.Type() == "identifier" || child.Type() == "use_wildcard" || child.Type() == "use_list" || child.Type() == "scoped_use_list" {
					result.Imports = append(result.Imports, Import{
						File: file,
						Path: nodeText(child, content),
					})
					break
				}
			}

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

func findRustImpl(n *sitter.Node, content []byte) string {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Type() == "impl_item" {
			if typeName := childByFieldName(p, "type"); typeName != nil {
				return nodeText(typeName, content)
			}
		}
	}
	return ""
}

func rustIsPublic(n *sitter.Node) bool {
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Type() == "visibility_modifier" {
			return true
		}
	}
	return false
}
