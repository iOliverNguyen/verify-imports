package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const FileName = ".import-restrictions"

type Rules struct {
	Rules []Rule
}

type Rule struct {
	SelectorRegexp    string
	AllowedPrefixes   []string
	ForbiddenPrefixes []string
}

type Verifier struct {
	base      string
	cfg       packages.Config
	mapPkgs   map[string]*packages.Package
	ruleFiles map[string]*Rules
}

func New(base string, dir string) *Verifier {
	if strings.HasSuffix(base, "/") {
		base = base[:len(base)-1]
	}
	cfg := packages.Config{
		Mode: packages.LoadImports,
		Dir:  dir,
	}
	return &Verifier{
		base:      base,
		cfg:       cfg,
		mapPkgs:   make(map[string]*packages.Package),
		ruleFiles: make(map[string]*Rules),
	}
}

func (v *Verifier) LoadPackages(patterns ...string) error {
	base2 := v.base + "/"
	for _, p := range patterns {
		if p != v.base && !strings.HasPrefix(p, base2) {
			return fmt.Errorf("pattern must start with base, but %q does not start with %q", p, base2)
		}
	}

	pkgs, err := packages.Load(&v.cfg, patterns...)
	if err != nil {
		return err
	}
	for _, pkg := range pkgs {
		v.mapPkgs[pkg.PkgPath] = pkg
	}
	return nil
}

func (v *Verifier) GetRuleFileForPackage(pkgPath string) (rules *Rules, err error) {
	// only return package under base path
	if !strings.HasPrefix(pkgPath, v.base) {
		return nil, nil
	}

	if rules, ok := v.ruleFiles[pkgPath]; ok {
		return rules, nil
	}
	defer func() {
		// cache the rule file, even if it's not found
		v.ruleFiles[pkgPath] = rules
	}()

	// try loading from disk
	relPath := pkgPath[len(v.base):]
	dirPath := filepath.Join(v.cfg.Dir, relPath)
	fi, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("directory %q not found: %v", dirPath, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("not a directory %q", dirPath)
	}

	path := filepath.Join(dirPath, FileName)
	if _, err = os.Stat(path); err == nil {
		return loadRuleFile(path)
	}
	return v.GetRuleFileForPackage(filepath.Dir(pkgPath))
}

func loadRuleFile(path string) (*Rules, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Rules
	err = json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}
	fmt.Println("loaded rules", path)
	return &r, nil
}

func (v *Verifier) VerifyPackage(pkg *packages.Package) (errs []error) {
	rules, err := v.GetRuleFileForPackage(pkg.PkgPath)
	if err != nil {
		return []error{err}
	}
	if rules == nil {
		return nil // skip the package
	}

	actualPath := filepath.Join(pkg.PkgPath, FileName)
	for _, r := range rules.Rules {
		re, err := regexp.Compile(r.SelectorRegexp)
		if err != nil {
			err = fmt.Errorf("regexp `%s` in file %q doesn't compile: %v", r.SelectorRegexp, actualPath, err)
			errs = append(errs, err)
			continue
		}
		for v := range pkg.Imports {
			if !re.MatchString(v) {
				continue
			}
			for _, forbidden := range r.ForbiddenPrefixes {
				if strings.HasPrefix(v, forbidden) {
					err = fmt.Errorf("import %q has forbidden prefix %v", v, forbidden)
					errs = append(errs, err)
					continue
				}
			}
			found := false
			for _, allowed := range r.AllowedPrefixes {
				if strings.HasPrefix(v, allowed) {
					found = true
					break
				}
			}
			if !found {
				err := fmt.Errorf("import %q did not match any allowed prefix", v)
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func (v *Verifier) Verify() error {
	paths := make([]string, 0, len(v.mapPkgs))
	for pkgPath := range v.mapPkgs {
		paths = append(paths, pkgPath)
	}
	sort.Strings(paths)

	ok := true
	for _, pkgPath := range paths {
		pkg := v.mapPkgs[pkgPath]
		errs := v.VerifyPackage(pkg)
		if errs != nil {
			ok = false
			fmt.Printf("Package %q\n", pkgPath)
			for i, err := range errs {
				fmt.Printf("\t%v\n", err)
				if i >= 9 && len(errs) > i {
					fmt.Printf("\t... total %v imports violated\n", len(errs))
					break
				}
			}
			fmt.Println()
		}
	}
	if !ok {
		return fmt.Errorf("some packages violate import rules")
	}
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Println(`Usage of verify-imports:
	verify-import -base BASE -dir DIR PATTERN ...

Example:
	verify-import -base github.com/me/myproject github.com/me/myproject/cmd/... github.com/me/myproject/pkg/...
`)
		flag.PrintDefaults()
	}

	cdir, err := os.Getwd()
	must("unexpected", err)
	flBase := flag.String("base", "", "Base package path (for example: github.com/me/myproject)")
	flDir := flag.String("dir", cdir, "The module directory (contains go.mod, default to working directory)")
	flag.Parse()
	patterns := flag.Args()

	if *flBase == "" || len(patterns) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	v := New(*flBase, *flDir)
	must("load packages:", v.LoadPackages(patterns...))
	must("verify imports:", v.Verify())
	fmt.Println("\n✓ ok")
}

func must(msg string, err error) {
	if err != nil {
		fmt.Println(msg, err)
		os.Exit(1)
	}
}
