package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func extractPython(root *sitter.Node, content []byte, file string) *ParseResult {
	result := &ParseResult{File: file}

	walkTree(root, func(n *sitter.Node) {
		switch n.Type() {
		case "function_definition":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			parent := findPythonClass(n, content)
			kind := "function"
			if parent != "" {
				kind = "method"
			}
			sig := nameStr
			if params := childByFieldName(n, "parameters"); params != nil {
				sig += nodeText(params, content)
			}
			if retType := childByFieldName(n, "return_type"); retType != nil {
				sig += " -> " + nodeText(retType, content)
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:      nameStr,
				Kind:      kind,
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Parent:    parent,
				Exported:  !pythonIsPrivate(nameStr),
			})

		case "class_definition":
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
				Exported: !pythonIsPrivate(nameStr),
			})

		case "import_statement":
			// import foo, bar
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "dotted_name" {
					result.Imports = append(result.Imports, Import{
						File: file,
						Path: nodeText(child, content),
					})
				}
			}

		case "import_from_statement":
			module := childByFieldName(n, "module_name")
			if module == nil {
				// Try to find dotted_name child
				for i := 0; i < int(n.ChildCount()); i++ {
					child := n.Child(i)
					if child.Type() == "dotted_name" || child.Type() == "relative_import" {
						module = child
						break
					}
				}
			}
			if module != nil {
				result.Imports = append(result.Imports, Import{
					File: file,
					Path: nodeText(module, content),
				})
			}

		case "call":
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

func findPythonClass(n *sitter.Node, content []byte) string {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Type() == "class_definition" {
			if name := childByFieldName(p, "name"); name != nil {
				return nodeText(name, content)
			}
		}
	}
	return ""
}

func pythonIsPrivate(name string) bool {
	return len(name) > 0 && name[0] == '_'
}
