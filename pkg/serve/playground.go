// Package serve provides unified GraphQL support for Petri-pilot services.
package serve

import (
	"net/http"
)

// PlaygroundHandler returns an HTTP handler that serves the GraphQL Playground UI.
func PlaygroundHandler(endpoint string) http.HandlerFunc {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>Petri-Pilot GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png" />
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root"></div>
  <script>
    window.addEventListener('load', function() {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '` + endpoint + `',
        settings: {
          'editor.theme': 'dark',
          'editor.fontFamily': "'Source Code Pro', 'Consolas', 'Inconsolata', 'Droid Sans Mono', 'Monaco', monospace",
          'editor.fontSize': 14,
          'request.credentials': 'same-origin'
        }
      })
    })
  </script>
</body>
</html>`

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}
}
