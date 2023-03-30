# Features

## Custom domains

...

## Redirects

Redirects can be created with a `_redirects` file with the following format:

```
# Comment
from  to  [status]
```

* Lines starting with `#` are ignored
* `from` - the path to redirect from (Note: repository and branch names are removed from request URLs)
* `to` - the path or URL to redirect to
* `status` - status code to use when redirecting (default 301)

### Status codes

* `200` - returns content from specified path (no external URLs) without changing the URL (rewrite)
* `301` - Moved Permanently (Permanent redirect)
* `302` - Found (Temporary redirect)

### Examples

#### SPA (single-page application) rewrite

Redirects all paths to `/index.html` for single-page apps.

```
/*  /index.html 200
```

#### Splats

Redirects every path under `/articles` to `/posts` while keeping the path.

```
/articles/*  /posts/:splat  302
```

Example: `/articles/2022/10/12/post-1/` -> `/posts/2022/10/12/post-1/`
