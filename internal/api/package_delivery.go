package api

import (
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
)

// packageVerified reports whether the caller of r has already proven
// whatever the package's verification_method requires. This is the single
// source of truth consulted both by GetPackagePreview (to decide whether to
// reveal the file list) and by GetShare (to gate the actual download) —
// duplicating this switch in both places would let them drift.
func (h *Handlers) packageVerified(r *http.Request, pkg db.Package) bool {
	switch pkg.VerificationMethod {
	case "none":
		return true
	case "bcc_login":
		caller := auth.CallerFrom(r.Context())
		return caller != nil && caller.Provider == "bcc"
	case "password":
		cookie, err := r.Cookie("pkg_verify_" + pkg.ID)
		if err != nil || cookie.Value == "" {
			return false
		}
		_, err = h.queries.GetValidPackageVerification(r.Context(), db.GetValidPackageVerificationParams{
			ID:        cookie.Value,
			PackageID: pkg.ID,
		})
		return err == nil
	default:
		// email_otp / magic_link: out of scope for now, nothing can satisfy them yet.
		return false
	}
}

type packageFileView struct {
	ShareID  string `json:"shareId"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type packagePreviewResponse struct {
	Name               string            `json:"name"`
	Message            string            `json:"message"`
	VerificationMethod string            `json:"verificationMethod"`
	ExpiresAt          string            `json:"expiresAt"`
	MaxDownloads       *int64            `json:"maxDownloads"`
	DownloadCount      int64             `json:"downloadCount"`
	Verified           bool              `json:"verified"`
	Files              []packageFileView `json:"files,omitempty"`
}

// GetPackagePreview is the public, unauthenticated endpoint a recipient's
// browser hits to see what a package contains. Before verification it only
// reveals enough to render a "verify" screen; the file list is withheld
// until packageVerified confirms the caller has satisfied the package's
// verification_method.
func (h *Handlers) GetPackagePreview(w http.ResponseWriter, r *http.Request) {
	packageID := r.PathValue("id")

	pkg, err := h.queries.GetPackageByID(r.Context(), packageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Package not found")
		return
	}

	if pkg.Status != "active" {
		writeJSONError(w, http.StatusGone, "Package has been revoked")
		return
	}

	if !pkg.ExpiresAt.After(time.Now()) {
		writeJSONError(w, http.StatusGone, "Package has expired")
		return
	}

	// Note: max_downloads is intentionally NOT checked here. Viewing the
	// preview is not a download, so it should still work even once the
	// download limit has been hit — each individual file download is
	// separately gated by GetShare.

	var maxDownloads *int64
	if pkg.MaxDownloads.Valid {
		maxDownloads = &pkg.MaxDownloads.Int64
	}

	resp := packagePreviewResponse{
		Name:               pkg.Name,
		Message:            pkg.Message,
		VerificationMethod: pkg.VerificationMethod,
		ExpiresAt:          pkg.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
		MaxDownloads:       maxDownloads,
		DownloadCount:      pkg.DownloadCount,
		Verified:           h.packageVerified(r, pkg),
	}

	if resp.Verified {
		rows, err := h.queries.ListSharesByPackageID(r.Context(), pkg.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to list package files")
			return
		}
		files := make([]packageFileView, len(rows))
		for i, row := range rows {
			files[i] = packageFileView{
				ShareID:  row.ShareID,
				Filename: row.Filename,
				Size:     row.Size,
			}
		}
		resp.Files = files
	}

	writeJSON(w, http.StatusOK, resp)
}

type verifyPackageRequest struct {
	Password string `json:"password"`
}

// VerifyPackage is the public endpoint a recipient's browser posts a
// password to. On success it issues an opaque, DB-backed verification
// token as a per-package cookie, mirroring auth.SessionStore's pattern
// (opaque token + DB row, expiry checked at lookup time). The same cookie
// subsequently satisfies both GetPackagePreview and GetShare via
// packageVerified.
func (h *Handlers) VerifyPackage(w http.ResponseWriter, r *http.Request) {
	packageID := r.PathValue("id")

	pkg, err := h.queries.GetPackageByID(r.Context(), packageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Package not found")
		return
	}

	if pkg.Status != "active" {
		writeJSONError(w, http.StatusGone, "Package has been revoked")
		return
	}

	if !pkg.ExpiresAt.After(time.Now()) {
		writeJSONError(w, http.StatusGone, "Package has expired")
		return
	}

	if pkg.VerificationMethod != "password" {
		writeJSONError(w, http.StatusBadRequest, "Package does not use password verification")
		return
	}

	var req verifyPackageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "Password is required")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(pkg.PasswordHash.String), []byte(req.Password)); err != nil {
		writeJSONError(w, http.StatusForbidden, "Incorrect password")
		return
	}

	token, err := generateShareID()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to generate verification token")
		return
	}

	if _, err := h.queries.CreatePackageVerification(r.Context(), db.CreatePackageVerificationParams{
		ID:        token,
		PackageID: pkg.ID,
		ExpiresAt: pkg.ExpiresAt,
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to record verification")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "pkg_verify_" + pkg.ID,
		Value:    token,
		Path:     "/",
		Expires:  pkg.ExpiresAt,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]bool{"verified": true})
}
