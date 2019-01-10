# verify-imports

This tool verifies imports between Go packages. It is written to support project
with go modules (and won't work with GOPATH). Inspired by
[import-boss](https://github.com/kubernetes/code-generator/tree/master/cmd/import-boss)
from [kubernetes](https://kubernetes.io/). Example is taken from
[kubernetes](https://github.com/kubernetes/code-generator/blob/master/cmd/import-boss/main.go).

It reads `.import-restrictions` file from each directory (if not found, looks in
parent directory recursively) and validate imports in the package against the
rules. An import is selected for each rule by `SelectorRegexp`. Then it is
allowed if it matches at least one allowed prefix and does not match any
forbidden prefix.

Example [.import-restrictions](https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/.import-restrictions):

```json
{
  "Rules": [
    {
      "SelectorRegexp": "k8s[.]io/kubernetes/pkg",
      "AllowedPrefixes": [
        "k8s.io/kubernetes/pkg/api"
      ],
      "ForbiddenPrefixes": [
        "k8s.io/kubernetes/pkg/api/deprecated"
      ]
    }
  ]
}
```

## Usage

Make sure your project use [go modules](https://github.com/golang/go/wiki/Modules).

Assume your project is "github.com/me/myproject" and has the following
structure:

```
myproject/
  cmd/
    mycommand/
      .import-restrictions    # apply to mycommand 
  pkg/
    foo/
    bar/
    .import-restrictions      # apply to pkg, pkg/foo, pkg/bar
```


```sh
# install verify-imports
go get github.com/ng-vu/verify-imports

# run it inside your project
cd myproject
verify-imports -base github.com/me/myproject github.com/me/myproject/...

# or list multiple patterns
verify-imports -base github.com/me/myproject github.com/me/myproject/cmd/... github.com/me/myproject/pkg/...
```

## License

[MIT](https://opensource.org/licenses/MIT)
