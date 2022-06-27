package dashboard

import "embed"

//go:generate npx tailwindcss -i styles.css -o assets/styles.css

//go:embed assets
var assetsFS embed.FS
