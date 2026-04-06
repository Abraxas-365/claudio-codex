package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func extractGo(root *sitter.Node, content []byte, file string) *ParseResult {
	result := &ParseResult{File: file}

	walkTree(root, func(n *sitter.Node) {
		switch n.Type() {
		case "function_declaration":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			sig := buildGoFuncSig(n, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:      nameStr,
				Kind:      "function",
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Exported:  isExported(nameStr),
			})

		case "method_declaration":
			name := childByFieldName(n, "name")
			if name == nil {
				return
			}
			nameStr := nodeText(name, content)
			recv := extractGoReceiver(n, content)
			sig := buildGoFuncSig(n, content)
			result.Symbols = append(result.Symbols, Symbol{
				Name:      nameStr,
				Kind:      "method",
				File:      file,
				Line:      int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: sig,
				Parent:    recv,
				Exported:  isExported(nameStr),
			})

		case "type_declaration":
			for i := 0; i < int(n.ChildCount()); i++ {
				spec := n.Child(i)
				if spec.Type() != "type_spec" {
					continue
				}
				typeName := childByFieldName(spec, "name")
				if typeName == nil {
					continue
				}
				nameStr := nodeText(typeName, content)
				kind := "type"
				typeNode := childByFieldName(spec, "type")
				if typeNode != nil {
					switch typeNode.Type() {
					case "struct_type":
						kind = "struct"
					case "interface_type":
						kind = "interface"
					}
				}
				result.Symbols = append(result.Symbols, Symbol{
					Name:     nameStr,
					Kind:     kind,
					File:     file,
					Line:     int(spec.StartPoint().Row) + 1,
					EndLine:  int(spec.EndPoint().Row) + 1,
					Exported: isExported(nameStr),
				})
			}

		case "const_declaration", "var_declaration":
			kind := "var"
			if n.Type() == "const_declaration" {
				kind = "const"
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				spec := n.Child(i)
				if spec.Type() != "const_spec" && spec.Type() != "var_spec" {
					continue
				}
				nameNode := childByFieldName(spec, "name")
				if nameNode == nil {
					continue
				}
				nameStr := nodeText(nameNode, content)
				result.Symbols = append(result.Symbols, Symbol{
					Name:     nameStr,
					Kind:     kind,
					File:     file,
					Line:     int(spec.StartPoint().Row) + 1,
					EndLine:  int(spec.EndPoint().Row) + 1,
					Exported: isExported(nameStr),
				})
			}

		case "import_declaration":
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				extractGoImportSpec(child, content, file, result)
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

func extractGoImportSpec(n *sitter.Node, content []byte, file string, result *ParseResult) {
	switch n.Type() {
	case "import_spec":
		pathNode := childByFieldName(n, "path")
		if pathNode == nil {
			return
		}
		path := strings.Trim(nodeText(pathNode, content), `"`)
		alias := ""
		if nameNode := childByFieldName(n, "name"); nameNode != nil {
			alias = nodeText(nameNode, content)
		}
		result.Imports = append(result.Imports, Import{
			File:  file,
			Path:  path,
			Alias: alias,
		})
	case "import_spec_list":
		for i := 0; i < int(n.ChildCount()); i++ {
			extractGoImportSpec(n.Child(i), content, file, result)
		}
	}
}

func extractGoReceiver(n *sitter.Node, content []byte) string {
	params := childByFieldName(n, "receiver")
	if params == nil {
		return ""
	}
	// Walk into parameter list to find the type
	for i := 0; i < int(params.ChildCount()); i++ {
		param := params.Child(i)
		if param.Type() == "parameter_declaration" {
			typeNode := childByFieldName(param, "type")
			if typeNode != nil {
				text := nodeText(typeNode, content)
				text = strings.TrimPrefix(text, "*")
				return text
			}
		}
	}
	return ""
}

func buildGoFuncSig(n *sitter.Node, content []byte) string {
	name := childByFieldName(n, "name")
	params := childByFieldName(n, "parameters")
	result := childByFieldName(n, "result")

	if name == nil {
		return ""
	}
	sig := nodeText(name, content)
	if params != nil {
		sig += nodeText(params, content)
	}
	if result != nil {
		sig += " " + nodeText(result, content)
	}
	return sig
}
