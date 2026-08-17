//go:build dev

package main

// devLinkOrigin is where a recipient opens a share link during development.
// The dev binary embeds no frontend (see embed_dev.go), so it serves API only
// and the SPA lives on the Vite dev server — port pinned in vite.config.ts.
const devLinkOrigin = "http://localhost:8091"
