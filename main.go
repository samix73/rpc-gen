package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

const clientTemplate = `
package {{.PackageName}}

func init() {
{{range .Methods}}
   gob.Register({{.ResponseType}}{})
   gob.Register({{.RequestType}}{})
{{end}}
}

type {{.ServiceName}}Client struct {
   client *rpc.Client
}

func New{{.ServiceName}}Client(address string) (*{{.ServiceName}}Client, error) {
   client, err := rpc.Dial("tcp", address)
   if err != nil {
       return nil, fmt.Errorf("{{$.PackageName}}.New{{.ServiceName}}Client rpc.Dial error: %w", err)
   }

   return &{{.ServiceName}}Client{client: client}, nil
}

{{range .Methods}}
func (c *{{$.ServiceName}}Client) {{.Name}}(request {{.RequestType}}) (*{{.ResponseType}}, error) {
   var response {{.ResponseType}}
   err := c.client.Call("{{$.ServiceName}}.{{.Name}}", request, &response)
   if err != nil {
       return nil, fmt.Errorf("{{$.PackageName}}.{{$.ServiceName}}Client.{{.Name}} Call error: %w", err)
   }

   return &response, nil
}
{{end}}

func (c *{{.ServiceName}}Client) Close() error {
   return c.client.Close()
}
`

type Method struct {
	Name         string
	RequestType  string
	ResponseType string
}

type ServiceData struct {
	PackageName string
	ServiceName string
	Methods     []Method
}

var (
	outputDir = flag.String("output", ".", "Output directory for generated files")
	verbose   = flag.Bool("verbose", false, "Enable verbose logging")
)

func log(format string, args ...interface{}) {
	if *verbose {
		slog.Info(fmt.Sprintf(format, args...))
	}
}

func extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + extractTypeName(t.X)
	default:
		return "unknown"
	}
}

func extractMethods(interfaceType *ast.InterfaceType) []Method {
	var methods []Method

	for _, method := range interfaceType.Methods.List {
		if funcType, ok := method.Type.(*ast.FuncType); ok {
			methodName := method.Names[0].Name

			// Extract request and response types from parameters
			if len(funcType.Params.List) >= 2 {
				requestType := extractTypeName(funcType.Params.List[0].Type)
				responseType := extractTypeName(funcType.Params.List[1].Type)

				// Remove pointer prefix from response type
				responseType = strings.TrimPrefix(responseType, "*")

				methods = append(methods, Method{
					Name:         methodName,
					RequestType:  requestType,
					ResponseType: responseType,
				})
			}
		}
	}

	return methods
}

func generateClientCode(temp *template.Template, serviceData ServiceData) error {
	buf := new(bytes.Buffer)
	if err := temp.Execute(buf, serviceData); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	formatted, err := imports.Process("", buf.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("imports error: %w", err)
	}

	fileName := fmt.Sprintf("%s_client_gen.go", strings.ToLower(serviceData.ServiceName))
	file, err := os.Create(path.Join(*outputDir, fileName))
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", fileName, err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(formatted); err != nil {
		return fmt.Errorf("error writing to file %s: %w", fileName, err)
	}

	return nil
}

func main() {
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(
		fset,
		wd,
		func(info fs.FileInfo) bool {
			return strings.HasSuffix(info.Name(), ".go") &&
				!strings.HasSuffix(info.Name(), "_test.go") &&
				!strings.HasSuffix(info.Name(), "_gen.go")
		},
		parser.ParseComments,
	)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	var serviceDatas []ServiceData

	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			log("Processing file: %s\n", fileName)

			ast.Inspect(file, func(n ast.Node) bool {
				if typeSpec, ok := n.(*ast.TypeSpec); ok {
					if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
						serviceName := typeSpec.Name.Name
						log("Found interface: %s\n", serviceName)

						// Extract methods from interface
						methods := extractMethods(interfaceType)

						serviceDatas = append(serviceDatas, ServiceData{
							PackageName: pkg.Name,
							ServiceName: serviceName,
							Methods:     methods,
						})
					}
				}

				return true
			})
		}
	}

	temp := template.New("clientTemplate")
	temp, err = temp.Parse(clientTemplate)
	if err != nil {
		slog.Error("Error parsing template", slog.String("error", err.Error()))
		os.Exit(1)
	}

	for _, serviceData := range serviceDatas {
		log("Generating client for service: %s\n", serviceData.ServiceName)

		if err := generateClientCode(temp, serviceData); err != nil {
			slog.Error("Error generating client code", slog.String("error", err.Error()))
			os.Exit(1)
		}

		log("Client code generated successfully for service: %s\n", serviceData.ServiceName)
	}
}
