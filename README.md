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

To use docsite for docs.sourcegraph.com, see the [docs.sourcegraph.com README](https://github.com/sourcegraph/docs.sourcegraph.com/blob/master/README.md).

## Site data

The `docsite` tool requires site data to be available in any of the following ways:

- A `docsite.json` file (or other file specified in the `-config` flag's search paths), as in the following example:
   ```json
   {
     "templates": "templates",
     "content": "../sourcegraph/doc",
     "baseURLPath": "/",
     "assets": "assets",
     "assetsBaseURLPath": "/assets/",
     "check": {
       "ignoreURLPattern": "(^https?://)|(^#)|(^mailto:support@sourcegraph\\.com$)|(^chrome://)"
     }
   }
   ```
- In the `DOCSITE_CONFIG` env var, using Zip archive URLs for `templates`, `assets`, and `content`, as in the following example:
   ```
   DOCSITE_CONFIG='{"templates":"https://codeload.github.com/sourcegraph/docs.sourcegraph.com/zip/master#docs.sourcegraph.com-master/templates/","assets":"https://codeload.github.com/sourcegraph/docs.sourcegraph.com/zip/master#docs.sourcegraph.com-master/assets/","content":"https://codeload.github.com/sourcegraph/sourcegraph/zip/master#sourcegraph-master/doc/","baseURLPath":"/","assetsBaseURLPath":"/assets/"}' docsite serve
   ```

## Development

### Release a new version

```shell
docker build -t sourcegraph/docsite . && \
docker push sourcegraph/docsite
```

> For internal Sourcegraph usage:
>   ``` shell
>   docker build -t sourcegraph/docsite . && \
>   docker tag sourcegraph/docsite us.gcr.io/sourcegraph-dev/docsite && \
>   docker push us.gcr.io/sourcegraph-dev/docsite
>   ```