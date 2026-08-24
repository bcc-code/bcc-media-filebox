//go:build !dev

package main

// Empty in production: guessing an origin would mail real recipients a link to
// the wrong host, and a sent mail can't be recalled. Set MAIL_LINK_BASE_URL or
// BASE_URL explicitly.
const devLinkOrigin = ""
