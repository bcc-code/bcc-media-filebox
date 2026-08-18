//go:build dev

package main

// Where a recipient opens a share link in development: the dev binary embeds no
// frontend (see embed_dev.go), so the SPA lives on the Vite dev server.
const devLinkOrigin = "http://localhost:8091"
