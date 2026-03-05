package codectx

import (
	"go/ast"
	"go/parser"
	"go/token"
)

type GoAnalyzer struct{}

func (GoAnalyzer) Analyze(path string) (imports []string, symbols []string) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil || file == nil {
		return
	}
	for _, im := range file.Imports {
		if im.Path != nil {
			v := im.Path.Value
			if len(v) >= 2 {
				imports = append(imports, v[1:len(v)-1])
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name != nil {
				symbols = append(symbols, x.Name.Name)
			}
		case *ast.TypeSpec:
			symbols = append(symbols, x.Name.Name)
		case *ast.ValueSpec:
			for _, id := range x.Names {
				symbols = append(symbols, id.Name)
			}
		}
		return true
	})
	return
}
