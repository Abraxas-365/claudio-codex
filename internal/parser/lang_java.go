package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"strings"
)

func extractJava(root *sitter.Node, content []byte, file string) *ParseResult {
	result := &ParseResult{File: file}

	walkTree(root, func(n *sitter.Node) {
		switch n.Type() {
		case "method_declaration":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			parent := findJavaClass(n, content)
			sig := nameStr
			if params := childByFieldName(n, "parameters"); params != nil {
				sig += nodeText(params, content)
			}
			if retType := childByFieldName(n, "type"); retType != nil {
				sig = nodeText(retType, content) + " " + sig
			}
			result.Symbols = append(result.Symbols, Symbol{
				Name:      nameStr,
				Kind:      "method",
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Parent:    parent,
				Exported:  javaIsPublic(n, content),
			})

		case "constructor_declaration":
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
				Kind:      "method",
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Parent:    findJavaClass(n, content),
				Exported:  javaIsPublic(n, content),
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
				Exported: javaIsPublic(n, content),
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
				Exported: javaIsPublic(n, content),
			})

		case "enum_declaration":
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
				Exported: javaIsPublic(n, content),
			})

		case "field_declaration":
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "variable_declarator" {
					vname := childByFieldName(child, "name")
					if vname == nil {
						continue
					}
					result.Symbols = append(result.Symbols, Symbol{
						Name:     nodeText(vname, content),
						Kind:     "var",
						File:     file,
						Line:     int(n.StartPoint().Row) + 1,
						EndLine:  int(n.EndPoint().Row) + 1,
						Parent:   findJavaClass(n, content),
						Exported: javaIsPublic(n, content),
					})
				}
			}

		case "import_declaration":
			text := nodeText(n, content)
			text = strings.TrimPrefix(text, "import ")
			text = strings.TrimPrefix(text, "static ")
			text = strings.TrimSuffix(text, ";")
			text = strings.TrimSpace(text)
			result.Imports = append(result.Imports, Import{
				File: file,
				Path: text,
			})

		case "method_invocation":
			name := childByFieldName(n, "name")
			obj := childByFieldName(n, "object")
			if name == nil {
				return
			}
			target := nodeText(name, content)
			if obj != nil {
				target = nodeText(obj, content) + "." + target
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

func findJavaClass(n *sitter.Node, content []byte) string {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Type() == "class_declaration" || p.Type() == "interface_declaration" || p.Type() == "enum_declaration" {
			if name := childByFieldName(p, "name"); name != nil {
				return nodeText(name, content)
			}
		}
	}
	return ""
}

func javaIsPublic(n *sitter.Node, content []byte) bool {
	if n.Type() == "method_declaration" || n.Type() == "constructor_declaration" || n.Type() == "field_declaration" {
		// Check for modifiers child
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "modifiers" {
				text := nodeText(child, content)
				return strings.Contains(text, "public")
			}
		}
		return false
	}
	// For class/interface/enum, check parent for modifiers
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Type() == "modifiers" {
			text := nodeText(child, content)
			return strings.Contains(text, "public")
		}
	}
	return false
}
