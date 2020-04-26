# docsite

A documentation site generator that fits [Sourcegraph](https://sourcegraph.com)'s needs:

- Markdown source files that are browseable on the file system and readable as plain text (without custom directives or complex front matter or configuration)
- Served by an HTTP server, not generated as static HTML files, to eliminate the need for external static site host configuration (which we found to be error-prone)
- Provides built-in site search for all documentation versions

## Usage

```shell
go get github.com/sourcegraph/docsite/cmd/docsite
docsite -h
```

- `docsite check`: check for common problems (such as broken links)
- `docsite serve`: serve the site over HTTP

To use docsite for docs.sourcegraph.com, see "[Documentation site](https://docs.sourcegraph.com/dev/documentation/site)" in the Sourcegraph documentation.

## Checks

The `docsite check` command runs various checks on your documentation site to find problems:

- invalid links
- broken links
- disconnected pages (with no inlinks from other pages)

If any problems are found, it exits with a non-zero status code.

To ignore the disconnected page check for a page, add YAML `ignoreDisconnectedPageCheck: true` to the top matter in the beginning of the `.md` file. For example:

```
---
ignoreDisconnectedPageCheck: true
---

# My page title
```

## Site data

The site data describes the location of its templates, assets, and content. It is a JSON object with the following properties.

- `content`: a VFS URL for the Markdown content files.
- `contentExcludePattern`: a regular expression specifying Markdown content files to exclude.
- `baseURLPath`: the URL path where the site is available (such as `/` or `/help/`).
- `templates`: a VFS URL for the [Go-style HTML templates](https://golang.org/pkg/html/template/) used to render site pages.
- `assets`: a VFS URL for the static assets referred to in the HTML templates (such as CSS stylesheets).
- `assetsBaseURLPath`: the URL path where the assets are available (such as `/assets/`).
- `redirects`: an object mapping URL paths (such as `/my/old/page`) to redirect destination URLs (such as `/my/new/page`).
- `check` (optional): an object containing a single property `ignoreURLPattern`, which is a [RE2 regexp](https://golang.org/pkg/regexp/syntax/) of URLs to ignore when checking for broken URLs with `docsite check`.

The possible values for VFS URLs are:

- A **relative path to a local directory** (such as `../myrepo/doc`). The path is interpreted relative to the `docsite.json` file (if it exists) or the current working directory (if site data is specified in `DOCSITE_CONFIG`).
- An **absolute URL to a Zip archive** (with `http` or `https` scheme). The URL can contain a fragment (such as `#mydir/`) to refer to a specific directory in the archive.

  If the URL fragment contains a path component `*` (such as `#*/templates/`), it matches the first top-level directory in the Zip file. (This is useful when using GitHub Zip archive URLs, such as `https://codeload.github.com/alice/myrepo/zip/myrev#*/templates/`. GitHub produces Zip archives with a top-level directory `$REPO-$REV`, such as `myrepo-myrev`, and using `#*/templates/` makes it easy to descend into that top-level directory without needing to duplicate the `myrev` in the URL fragment.)

  If the URL contains the literal string `$VERSION`, it is replaced by the user's requested version from the URL (e.g., the URL path `/@foo/bar` means the version is `foo`). ⚠️ If you are using GitHub `codeload.github.com` archive URLs, be sure your URL contains `refs/heads/$VERSION` (as in `https://codeload.github.com/owner/repo/zip/refs/heads/$VERSION`), not just `$VERSION`. This prevents someone from forking your repository, pushing a commit to their fork with unauthorized content, and then crafting a URL on your documentation site that would cause users to view that unauthorized content (which may contain malicious scripts or misleading information).

### Templates

The templates use [Go-style HTML templates](https://golang.org/pkg/html/template/).

- Document pages are rendered using a template named `document.html`.
- Search result pages are rendered using a template named `search.html`.
- The file `root.html`, if it exists, is loaded when rendering any template. You can define common templates in this file.

See the following examples:

- [about.sourcegraph.com/handbook templates](https://github.com/sourcegraph/about/tree/master/_resources/templates)
- [docs.sourcegraph.com templates](https://github.com/sourcegraph/sourcegraph/tree/master/doc/_resources/templates)

### Redirects

In addition to the `redirects` property in site data, you can also specify redirects in a text file named `redirects` at the top level of the `assets` VFS. The format is as follows:

``` text
FROM-PATH TO-URL STATUS-CODE
```

For example:

``` text
# Comments are allowed
/my/old/page /my/new/page 308
/another/page https://example.com/page 308
```

### Specifying site data

The `docsite` tool requires site data to be available in any of the following ways:

- A `docsite.json` file (or other file specified in the `-config` flag's search paths), as in the following example:
   ```json
   {
     "content": "../sourcegraph/doc",
     "baseURLPath": "/",
     "templates": "templates",
     "assets": "assets",
     "assetsBaseURLPath": "/assets/",
     "check": {
       "ignoreURLPattern": "(^https?://)|(^#)|(^mailto:support@sourcegraph\\.com$)|(^chrome://)"
     }
   }
   ```
- In the `DOCSITE_CONFIG` env var, using Zip archive URLs for `templates`, `assets`, and `content`, as in the following example:
   ```
   DOCSITE_CONFIG='{"templates":"https://codeload.github.com/sourcegraph/sourcegraph/zip/refs/heads/master#*/doc/_resources/templates/","assets":"https://codeload.github.com/sourcegraph/sourcegraph/zip/refs/heads/master#*/doc/_resources/assets/","content":"https://codeload.github.com/sourcegraph/sourcegraph/zip/refs/heads/$VERSION#*/doc/","baseURLPath":"/","assetsBaseURLPath":"/assets/"}' docsite serve
   ```

## Development

### Release a new version

```shell
docker build -t sourcegraph/docsite . && \
docker push sourcegraph/docsite
```

For internal Sourcegraph usage, then bump the deployed version by updating the SHA-256 image digest in all files that refer to `sourcegraph/docsite:latest@sha256:...`. Currently the 2 files that need to be updated are `configure/about-sourcegraph-com/{docs,about}-sourcegraph-com.Deployment.yaml`.
