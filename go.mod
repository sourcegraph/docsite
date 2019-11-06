module github.com/sourcegraph/docsite

require (
	github.com/Depado/bfchroma v1.1.1
	github.com/alecthomas/chroma v0.6.2
	github.com/alecthomas/repr v0.0.0-20181024024818-d37bc2a10ba1 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mozillazg/go-slugify v0.2.0
	github.com/mozillazg/go-unidecode v0.1.1 // indirect
	github.com/pkg/errors v0.8.1
	github.com/shurcooL/sanitized_anchor_name v1.0.0
	github.com/sourcegraph/go-jsonschema v0.0.0-20190205151546-7939fa138765
	github.com/sourcegraph/jsonschemadoc v0.0.0-20190214000648-1850b818f08c
	github.com/stretchr/testify v1.3.0 // indirect
	golang.org/x/net v0.0.0-20190110200230-915654e7eabc
	golang.org/x/sys v0.0.0-20190109145017-48ac38b7c8cb // indirect
	golang.org/x/tools v0.0.0-20190110211028-68c5ac90f574
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/russross/blackfriday.v2 v2.0.1
	gopkg.in/yaml.v2 v2.2.2
)

replace gopkg.in/russross/blackfriday.v2 v2.0.1 => github.com/russross/blackfriday/v2 v2.0.1

go 1.13
