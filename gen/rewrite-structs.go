package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/crosbymichael/upgrade/srcimporter"
)

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	filename := os.Getenv("GOFILE")
	goLine, err := strconv.Atoi(os.Getenv("GOLINE"))
	if err != nil {
		fatal(err)
	}
	typeName, err := parseTypeName(filename, goLine)
	if err != nil {
		fatal(err)
	}

	vndr, err := readVendor()
	if err != nil {
		fatal(err)
	}

	pkgName := os.Getenv("GOPACKAGE")

	args := os.Args[1:]
	if args[0] == "--" {
		args = args[1:]
	}
	content, err := generate(args[1:], filename, pkgName, typeName, vndr)
	if err != nil {
		fatal(err)
	}

	newFilename := args[0]
	if err := ioutil.WriteFile(newFilename, content, 0644); err != nil {
		fatal(err)
	}
}

type vendor map[string]struct {
	ref  string
	fork string
}

func readVendor() (vendor, error) {
	fi1, err := os.Stat("../vendor.conf")
	if err != nil {
		return nil, err
	}
	f, err := os.Open("vendor.conf")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi2, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(fi1, fi2) {
		return nil, fmt.Errorf("Error: you must make top-level vendor.conf a symlink to the current directory's vendor.conf and re-run vndr")
	}

	scanner := bufio.NewScanner(f)
	vndr := make(vendor)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if len(t) == 0 || t[0] == '#' {
			continue
		}
		fields := strings.Fields(t)
		fork := ""
		if len(fields) > 2 {
			fork = fields[2]
		}
		vndr[fields[0]] = struct{ ref, fork string }{fields[1], fork}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return vndr, nil
}

func parseTypeName(filename string, goLine int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() && line < goLine {
		line++
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if line != goLine {
		return "", fmt.Errorf("reached EOF before reaching line %d in %s", goLine, filename)
	}
	fields := strings.Fields(scanner.Text())
	if fields[0] != "type" {
		return "", fmt.Errorf("The `//go:generate ...` line should be directly above a `type Somename Sometype` declaration")
	}
	return fields[1], nil
}

type prefix map[string]string

type rewriter struct {
	m                  prefix
	pkg                *types.Package
	numPtr             int
	imports            map[string]*types.Package
	userDefinedImports map[string]string
}

func (r *rewriter) ensurePointers(buf *bytes.Buffer, anonymous bool) {
	if r.numPtr > 0 {
		if !anonymous {
			for i := 0; i < r.numPtr; i++ {
				buf.WriteByte('*')
			}
		}
		r.numPtr = 0
	}
}

func (r *rewriter) writeType(buf *bytes.Buffer, fieldPath string, anonymous bool, t types.Type) {
	rewritten, unrolling := r.m[fieldPath]
	if unrolling && rewritten != "" {
		r.ensurePointers(buf, anonymous)
		buf.WriteString(rewritten)
		return
	}
	if !anonymous && !unrolling {
		if t, ok := t.(*types.Named); ok {
			r.ensurePointers(buf, anonymous)
			types.WriteType(buf, t, func(p *types.Package) string {
				r.imports[p.Path()] = p
				if name, ok := r.userDefinedImports[p.Path()]; ok {
					return name
				}
				return p.Name()
			})
			return
		}
	}

	// Pointer case is a special case because of embedded pointers to structs.
	// We'd want to call ensurePointers for all other types than Pointer.
	if x, isPtr := t.Underlying().(*types.Pointer); isPtr {
		r.numPtr++
		r.writeType(buf, fieldPath, anonymous, x.Elem())
		return
	}
	r.ensurePointers(buf, anonymous)

	switch x := t.Underlying().(type) {
	case *types.Struct:
		if !anonymous {
			buf.WriteString("struct{")
		}
		for i := 0; i < x.NumFields(); i++ {
			f := x.Field(i)
			anonymous := f.Anonymous()
			if !anonymous {
				buf.WriteString(f.Name())
				buf.WriteByte(' ')
			}
			newFieldPath := fieldPath
			if !anonymous {
				newFieldPath = fmt.Sprintf("%s.%s", fieldPath, f.Name())
			}
			r.writeType(buf, newFieldPath, anonymous, f.Type())
			if !anonymous {
				if tag := x.Tag(i); tag != "" {
					if strings.Index(tag, "`") < 0 {
						buf.WriteString(" `")
						buf.WriteString(tag)
						buf.WriteByte('`')
					} else {
						fmt.Fprintf(buf, "%q", tag)
					}
				}
				buf.WriteByte(';')
			}
		}
		if !anonymous {
			buf.WriteByte('}')
		}
	case *types.Array:
		buf.WriteString(fmt.Sprintf("[%d]", x.Len()))
		r.writeType(buf, fieldPath, anonymous, x.Elem())
	case *types.Slice:
		buf.WriteString("[]")
		r.writeType(buf, fieldPath, anonymous, x.Elem())
	case *types.Basic:
		buf.WriteString(x.String())
	case *types.Map:
		buf.WriteString("map[")
		r.writeType(buf, fieldPath, anonymous, x.Key())
		buf.WriteByte(']')
		r.writeType(buf, fieldPath, anonymous, x.Elem())
	default:
		fmt.Println(buf.String())
		panic(fmt.Errorf("type %T not implemented, please fix", t))
	}
}

func index(s, sep string) int {
	i := strings.Index(s, sep)
	if i < 0 {
		return len(s)
	}
	return i
}

func generate(rules []string, filename, pkgName, typeName string, vndr vendor) ([]byte, error) {
	fset := token.NewFileSet() // positions are relative to fset

	// Parse the file containing this very example
	// but stop after processing the imports.
	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}

	conf := types.Config{Importer: srcimporter.New(&build.Default, token.NewFileSet(), make(map[string]*types.Package))}
	pkg, err := conf.Check(pkgName, fset, []*ast.File{f}, nil)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	buf.WriteString("type ")
	buf.WriteString(typeName)
	buf.WriteByte(' ')

	pfx := make(prefix, len(rules))
	for _, rule := range rules {
		i := strings.Index(rule, "->")
		if i < 0 {
			return nil, fmt.Errorf("expecting rewrite rule of the form .Path.To.Struct.Field->newName, got: %s", rule)
		}
		z := "."
		parts := strings.FieldsFunc(rule[:i], func(r rune) bool { return r == '.' })
		if len(parts) > 0 {
			z += parts[0]
			parts = parts[1:]
			pfx[z] = ""
		}
		for _, part := range parts {
			z += "." + part
			pfx[z] = ""
		}
		pfx[z] = rule[i+2:]
	}
	r := &rewriter{m: pfx, pkg: pkg, imports: make(map[string]*types.Package, len(pkg.Imports())), userDefinedImports: make(map[string]string)}

	for _, imp := range pkg.Imports() {
		r.userDefinedImports[imp.Path()] = imp.Name()
	}

	t := pkg.Scope().Lookup(typeName).Type().Underlying()
	r.writeType(buf, "", false, t)

	thirdPartyImports := make([]*types.Package, 0, len(r.imports))
	stdImports := []*types.Package{}
	for _, imp := range r.imports {
		path := imp.Path()
		if strings.Index(path[:index(path, "/")], ".") < 0 {
			stdImports = append(stdImports, imp)
		} else {
			thirdPartyImports = append(thirdPartyImports, imp)
		}
	}
	for _, imports := range [][]*types.Package{stdImports, thirdPartyImports} {
		sort.Slice(stdImports, func(i, j int) bool {
			return imports[i].Path() < imports[j].Path()
		})
	}

	finalBuf := bytes.NewBufferString(`// DO NOT EDIT
// This file has been auto-generated with go generate.

package `)
	finalBuf.WriteString(pkgName)
	finalBuf.WriteString(`
import `)
	moreThanOneImport := len(stdImports)+len(thirdPartyImports) > 1
	if moreThanOneImport {
		finalBuf.WriteString("(\n")
	}
	writeImport := func(imp *types.Package) {
		path := imp.Path()
		if v := strings.LastIndex(path, "/vendor/"); v >= 0 {
			path = path[v+len("/vendor/"):]
		}
		comment := ""
		for v, s := range vndr {
			if strings.HasPrefix(path, v) {
				comment = " // " + strings.Join([]string{s.ref, s.fork}, " ")
			}
		}
		if i := strings.LastIndex(path, "/"); i >= 0 && path[i+1:] != imp.Name() {
			fmt.Fprintf(finalBuf, "%s %q%s\n", imp.Name(), path, comment)
		} else {
			fmt.Fprintf(finalBuf, "%q%s\n", path, comment)
		}
	}

	for _, imp := range stdImports {
		writeImport(imp)
	}
	if len(stdImports) > 0 && len(thirdPartyImports) > 0 {
		finalBuf.WriteByte('\n')
	}
	for _, imp := range thirdPartyImports {
		writeImport(imp)
	}
	if moreThanOneImport {
		finalBuf.WriteString(")\n")
	} else {
		finalBuf.WriteByte('\n')
	}
	io.Copy(finalBuf, buf)

	pretty, err := format.Source(finalBuf.Bytes())
	if err != nil {
		fmt.Println(finalBuf.String())
		return nil, err
	}

	return pretty, nil
}
