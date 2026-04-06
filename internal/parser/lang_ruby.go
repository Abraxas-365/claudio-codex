package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func extractRuby(root *sitter.Node, content []byte, file string) *ParseResult {
	result := &ParseResult{File: file}

	walkTree(root, func(n *sitter.Node) {
		switch n.Type() {
		case "method":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			parent := findRubyClass(n, content)
			sig := nameStr
			if params := childByFieldName(n, "parameters"); params != nil {
				sig += nodeText(params, content)
			}
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
				Exported:  !rubyIsPrivate(nameStr),
			})

		case "singleton_method":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			parent := findRubyClass(n, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "method",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Parent:   parent,
				Exported: true,
			})

		case "class":
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

		case "module":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:     nameStr,
				Kind:     "module",
				File:     file,
				Line:     int(n.StartPoint().Row) + 1,
				EndLine:  int(n.EndPoint().Row) + 1,
				Exported: true,
			})

		case "constant_assignment":
			// FOO = ...
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "constant" {
					result.Symbols = append(result.Symbols, Symbol{
						Name:     nodeText(child, content),
						Kind:     "const",
						File:     file,
						Line:     int(n.StartPoint().Row) + 1,
						EndLine:  int(n.EndPoint().Row) + 1,
						Exported: true,
					})
					break
				}
			}

		case "call":
			methodNode := childByFieldName(n, "method")
			if methodNode == nil {
				return
			}
			target := nodeText(methodNode, content)
			if recv := childByFieldName(n, "receiver"); recv != nil {
				target = nodeText(recv, content) + "." + target
			}

			// Check if this is a require/require_relative
			methodText := nodeText(methodNode, content)
			if methodText == "require" || methodText == "require_relative" {
				if args := childByFieldName(n, "arguments"); args != nil {
					for i := 0; i < int(args.ChildCount()); i++ {
						arg := args.Child(i)
						if arg.Type() == "string" {
							path := trimQuotes(nodeText(arg, content))
							result.Imports = append(result.Imports, Import{
								File: file,
								Path: path,
							})
							return
						}
					}
				}
			}

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

func findRubyClass(n *sitter.Node, content []byte) string {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Type() == "class" || p.Type() == "module" {
			if name := childByFieldName(p, "name"); name != nil {
				return nodeText(name, content)
			}
		}
	}
	return ""
}

func rubyIsPrivate(name string) bool {
	return len(name) > 0 && name[0] == '_'
}
