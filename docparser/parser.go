package docparser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
)

func parseFile(path string) (*ast.File, error) {
	data, err := ioutil.ReadFile(path) // just pass the file name
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet() // positions are relative to fset
	return parser.ParseFile(fset, "", data, parser.ParseComments)
}

func parseJSONTag(field *ast.Field) (name string, ignore bool, required bool, err error) {
	if len(field.Names) > 0 {
		name = field.Names[0].Name
	}
	if field.Tag != nil && len(strings.TrimSpace(field.Tag.Value)) > 0 {
		tv, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			return name, false, false, err
		}

		if strings.TrimSpace(tv) != "" {
			st := reflect.StructTag(tv)
			jsonName := strings.Split(st.Get("json"), ",")[0]
			if jsonName == "-" {
				return name, true, false, nil
			} else if jsonName != "" {
				required := false
				// https://github.com/go-playground/validator
				// check if validate attr is active
				validateData := strings.Split(st.Get("validate"), ",")
				for _, v := range validateData {
					if v == "required" {
						required = true
					}
				}
				return jsonName, false, required, nil
			}
		}
	}
	return name, false, false, nil
}

func parseNamedType(gofile *ast.File, expr ast.Expr) (*property, error) {
	p := property{}
	switch ftpe := expr.(type) {
	case *ast.Ident: // simple value
		t, format, err := parseIdentProperty(ftpe)
		if err != nil {
			p.Ref = "#/components/schemas/" + t
			return &p, nil
		}
		p.Type = t
		p.Format = format
		return &p, nil
	case *ast.StarExpr: // pointer to something, optional by default
		t, _ := parseNamedType(gofile, ftpe.X)
		t.Nullable = true
		return t, nil
	case *ast.ArrayType: // slice type
		cp, _ := parseNamedType(gofile, ftpe.Elt)
		p.Type = "array"
		p.Items = map[string]string{}
		if cp.Type != "" {
			p.Items["type"] = cp.Type
		}
		if cp.Ref != "" {
			p.Items["$ref"] = cp.Ref
		}
		return &p, nil
	case *ast.StructType:
		return nil, fmt.Errorf("expr (%s) not yet unsupported", expr)
	case *ast.SelectorExpr:
		// @TODO ca va bugger ici !
		t, _ := parseNamedType(gofile, ftpe.X)
		return t, nil
	case *ast.MapType:
		return nil, fmt.Errorf("expr (%s) not yet unsupported", expr)
	case *ast.InterfaceType:
		return nil, fmt.Errorf("expr (%s) not yet unsupported", expr)
	default:
		return nil, fmt.Errorf("expr (%s) type (%s) is unsupported for a schema", ftpe, expr)
	}
}

// https://swagger.io/specification/#dataTypes
func parseIdentProperty(expr *ast.Ident) (t, format string, err error) {
	switch expr.Name {
	case "string":
		t = "string"
	case "bson":
		t = "string"
	case "int":
		t = "integer"
	case "int64":
		t = "integer"
	case "int32":
		t = "integer"
	case "time":
		t = "string"
		format = "date-time"
	case "float64":
		t = "number"
	case "bool":
		t = "boolean"
	default:
		t = expr.Name
		err = fmt.Errorf("Can't set the type %s", expr.Name)
	}
	return t, format, err
}
