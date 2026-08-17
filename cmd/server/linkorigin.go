//go:build !dev

package main

// devLinkOrigin is empty in production builds: guessing an origin would mail
// real recipients a link to the wrong host, and a sent mail cannot be recalled.
// MAIL_LINK_BASE_URL or BASE_URL must be set explicitly.
const devLinkOrigin = ""
