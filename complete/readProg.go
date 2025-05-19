// Copyright (c) 2024 Erik Kassubek
//
// File: readProg.go
// Brief: Functions to read in a program an extract all relevant operations
//
// Author: Erik Kassubek
// Created: 2024-06-26
//
// License: BSD-3-Clause

package complete

import (
	"advocate/utils"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

// Pass over all go files in a program and get the positions of all relevant
// primitives and operation on those primitives
//
// Parameter:
//   - progPath string: path to the project
//
// Returns:
//   - map[string][]int: all lines in the code that contain relevant operations.
//   - The map has the form filePath -> list of lines in this file
//   - error
func getProgramElements(progPath string) (map[string][]int, error) {
	progElems := make(map[string][]int)

	files, err := collectGoFiles(progPath)
	if err != nil {
		utils.LogError("Error in collecting files")
		return nil, err
	}

	pkg, err := analyzeFiles(files)
	if err != nil {
		utils.LogError("Error in analyzing files")
		return nil, err
	}

	// traverse all .go files in the directory recursively
	err = filepath.Walk(progPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			utils.LogError("Error in walking")
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".go") {
			content, err := os.ReadFile(path)
			if err != nil {
				utils.LogError("Error in reading file")
				return err
			}

			elems, err := getElemsFromContent(path, string(content), pkg)

			if err == nil && len(elems) > 0 {
				if _, ok := progElems[path]; !ok {
					progElems[path] = make([]int, 0)
				}
				for _, elem := range elems {
					progElems[path] = append(progElems[path], elem)
				}
			}
		}
		return nil
	})

	return progElems, err
}

// Given a directory, recursively collect all go files
//
// Parameter:
//   - dir string: path to the directory
//
// Returns:
//   - []string: paths to all go file in dir
//   - error
func collectGoFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			utils.LogErrorf("Error when collecting %q: %v\n", path, err)
			return err
		}
		if info == nil {
			return nil
		}

		if info.IsDir() && info.Name() == "advocateResult" {
			return filepath.SkipDir
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// analyzeFiles type-checks a package and returns the resulting package object
//
// Parameter:
//   - files []string: list of files to analyze
//
// Returns:
//   - *types.Package: the resulting types package
//   - error
func analyzeFiles(files []string) (*types.Package, error) {
	fset := token.NewFileSet()
	var astFiles []*ast.File

	for _, file := range files {
		parsedFile, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			continue
		}
		astFiles = append(astFiles, parsedFile)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, _ := conf.Check("mypackage", fset, astFiles, &types.Info{
		Uses: make(map[*ast.Ident]types.Object),
	})

	return pkg, nil
}

// getElemsFromContent passes the ast tree and for a given file, finds the
// line numbers of all relevant elements
//
// Parameter:
//   - path string: the path to the analyzed file
//   - content string: the content of the file
//   - pkg *types.Package: The type information for the ast
//
// Returns:
//   - []int: the line numbers containing relevant operations
//   - error
func getElemsFromContent(path string, content string, pkg *types.Package) ([]int, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return []int{}, err
	}

	info := &types.Info{
		Uses: make(map[*ast.Ident]types.Object),
	}

	imports := pkg.Imports()

	var syncPkg *types.Package
	for _, imp := range imports {
		if imp.Name() == "sync" {
			syncPkg = imp
			break
		}
	}

	v := &visitor{fset: fset, pkg: pkg, info: info, syncPkg: syncPkg,
		selectCases: make(map[string]struct{}), elements: make([]int, 0)}
	ast.Walk(v, node)

	return v.elements, nil
}

// visitor ist eine Struktur, die das ast.Visitor Interface implementiert.
// Sie wird verwendet, um den AST zu durchlaufen.
type visitor struct {
	fset        *token.FileSet
	pkg         *types.Package
	info        *types.Info
	syncPkg     *types.Package
	selectCases map[string]struct{}
	elements    []int // line numbers
}

// Visit is called for each node when passing the ast. It determines if the
// node represents a relevant operations and if so records the line number
// of the element
//
// Parameter:
//   - n ast.Node: the currently visited node
//
// Returns:
//   - ast.Visitor
func (v *visitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}

	switch x := n.(type) {
	case *ast.GoStmt:
		v.recordElement(v.fset.Position(n.Pos()))
	case *ast.SendStmt: // send
		if _, ok := v.selectCases[v.fset.Position(n.Pos()).String()]; ok {
			delete(v.selectCases, v.fset.Position(n.Pos()).String())
		} else {
			v.recordElement(v.fset.Position(n.Pos()))
		}
	case *ast.UnaryExpr: // recv
		if x.Op == token.ARROW {
			if _, ok := v.selectCases[v.fset.Position(n.Pos()).String()]; ok {
				delete(v.selectCases, v.fset.Position(n.Pos()).String())
			} else {
				v.recordElement(v.fset.Position(n.Pos()))
			}
		}
	case *ast.CallExpr:
		// close
		if fun, ok := x.Fun.(*ast.Ident); ok && fun.Name == "close" {
			v.recordElement(v.fset.Position(n.Pos()))
		}
		if fun, ok := x.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := fun.X.(*ast.Ident); ok {
				obj := v.info.Uses[ident]

				if obj != nil && v.syncPkg != nil {
					typ := obj.Type()

					// Überprüfen Sie, ob der Typ zu einem der spezifischen Typen gehört
					mutexType := v.syncPkg.Scope().Lookup("Mutex").Type()
					rwMutexType := v.syncPkg.Scope().Lookup("RWMutex").Type()
					wgType := v.syncPkg.Scope().Lookup("WaitGroup").Type()
					condType := v.syncPkg.Scope().Lookup("Cond").Type()
					onceType := v.syncPkg.Scope().Lookup("Once").Type()

					switch {
					case types.AssignableTo(typ, mutexType):
						v.recordElement(v.fset.Position(n.Pos()))
					case types.AssignableTo(typ, rwMutexType):
						v.recordElement(v.fset.Position(n.Pos()))
					case types.AssignableTo(typ, wgType):
						v.recordElement(v.fset.Position(n.Pos()))
					case types.AssignableTo(typ, condType):
						v.recordElement(v.fset.Position(n.Pos()))
					case types.AssignableTo(typ, onceType):
						v.recordElement(v.fset.Position(n.Pos()))
					}
				}
			}
		}
	case *ast.SelectStmt:
		v.recordElement(v.fset.Position(n.Pos()))
		for _, stmt := range x.Body.List {
			caseClause, ok := stmt.(*ast.CommClause)
			if !ok {
				continue // Nicht ein case-timeouteil, weitermachen
			}
			switch comm := caseClause.Comm.(type) {
			case *ast.SendStmt:
				// store to not record the send statement
				v.selectCases[v.fset.Position(comm.Pos()).String()] = struct{}{}
			case *ast.ExprStmt:
				// store to not record the recv statement
				if unaryExpr, ok := comm.X.(*ast.UnaryExpr); ok && unaryExpr.Op == token.ARROW {
					v.selectCases[v.fset.Position(unaryExpr.Pos()).String()] = struct{}{}
				}
			}
		}
	case *ast.RangeStmt:
		rangeExpr := x.X
		rangeExprType := v.info.Types[rangeExpr].Type
		// Check if the range expression is a channel
		if _, ok := rangeExprType.(*types.Chan); ok {
			fmt.Printf("Range über Kanal gefunden bei %s\n", v.fset.Position(n.Pos()))
		}
	}

	return v
}

// recordElement stores the line of a node in the visitor
//
// Parameter:
//   - pos (token.Position): the code position of the node for which the
//     function is called
func (v *visitor) recordElement(pos token.Position) {
	v.elements = append(v.elements, pos.Line)
}
