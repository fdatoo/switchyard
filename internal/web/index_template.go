package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"strings"
)

const indexTemplateSource = `<!doctype html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>gohome</title>
<meta name="gohome-version" content="{{.Version}}">
<script>
(function(){try{var m=localStorage.getItem('gohome.themeMode')||'system';var r=m==='system'?(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'):m;document.documentElement.setAttribute('data-theme','developer-'+r);}catch(_){}})();
</script>
{{.AssetTags}}
</head>
<body>
<div id="root"></div>
{{.ScriptTags}}
</body>
</html>
`

func renderIndex(version string, dist fs.FS) ([]byte, error) {
	assetTags, scriptTags, err := scanDist(dist)
	if err != nil {
		return nil, fmt.Errorf("web: scan dist: %w", err)
	}
	tmpl, err := template.New("index").Parse(indexTemplateSource)
	if err != nil {
		return nil, fmt.Errorf("web: parse index template: %w", err)
	}
	var buf bytes.Buffer
	data := struct {
		Version    string
		AssetTags  template.HTML
		ScriptTags template.HTML
	}{
		Version:    version,
		AssetTags:  template.HTML(assetTags),
		ScriptTags: template.HTML(scriptTags),
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("web: execute index template: %w", err)
	}
	return buf.Bytes(), nil
}

func scanDist(dist fs.FS) (assetTags, scriptTags string, err error) {
	f, err := dist.Open("index.html")
	if err != nil {
		return "", "", err
	}
	defer f.Close() //nolint:errcheck

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(f); err != nil {
		return "", "", err
	}

	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.Contains(line, `rel="stylesheet"`) && strings.Contains(line, "/assets/"):
			assetTags += line + "\n"
		case strings.HasPrefix(line, `<script type="module"`) && strings.Contains(line, "/assets/"):
			scriptTags += line + "\n"
		}
	}
	return assetTags, scriptTags, nil
}
