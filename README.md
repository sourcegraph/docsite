# docsite

A documentation site generator that fits [Sourcegraph](https://sourcegraph.com)'s needs:

- Markdown source files that are browseable on the file system and readable as plain text (without custom directives or complex front matter or configuration)
- Served by an HTTP server, not generated as static HTML files, to eliminate the need for external static site host configuration
- Usable by Sourcegraph itself as a library, to self-host docs for the current product version

## Usage

```shell
go get github.com/sourcegraph/docsite/cmd/docsite
docsite -h
```

- `docsite check`: check for common problems (such as broken links)
- `docsite serve`: serve the site over HTTP
- `docsite build`: bundle site data into the ELF executable to make a standalone program that can serve the site

To use docsite for docs.sourcegraph.com, see the [docs.sourcegraph.com README](https://github.com/sourcegraph/docs.sourcegraph.com/blob/master/README.md).