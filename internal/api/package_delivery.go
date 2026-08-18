package api

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/mail"
)

// packageVerified reports whether r's caller has satisfied the package's
// verification_method. Shared by GetPackagePreview and GetShare, which must
// agree on what counts as verified.
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
		// email_otp / magic_link aren't implemented yet.
		return false
	}
}

type packageFileView struct {
	ShareID     string `json:"shareId"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	AccessCount int64  `json:"accessCount"`
}

type packagePreviewResponse struct {
	Name               string            `json:"name"`
	SenderName         string            `json:"senderName"`
	Message            string            `json:"message"`
	VerificationMethod string            `json:"verificationMethod"`
	ExpiresAt          string            `json:"expiresAt"`
	MaxDownloads       *int64            `json:"maxDownloads"`
	DownloadCount      int64             `json:"downloadCount"`
	Verified           bool              `json:"verified"`
	Files              []packageFileView `json:"files,omitempty"`
}

// GetPackagePreview is the public endpoint a recipient's browser hits to see
// what a package contains. The file list is withheld until packageVerified
// passes; before that it reveals only enough to render the verify screen.
func (h *Handlers) GetPackagePreview(w http.ResponseWriter, r *http.Request) {
	packageID := r.PathValue("id")

	pkg, err := h.queries.GetPackageByID(r.Context(), packageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, errPackageNotFound)
		return
	}

	// unavailableReason is passed nil so it skips the download limit: viewing
	// isn't downloading, and GetShare gates each file separately.
	if reason := unavailableReason(pkg, nil); reason != "" {
		h.writePackageUnavailable(w, r, pkg, reason)
		return
	}

	var maxDownloads *int64
	if pkg.MaxDownloads.Valid {
		maxDownloads = &pkg.MaxDownloads.Int64
	}

	maxAccessCount, err := h.queries.GetPackageMaxAccessCount(r.Context(), pkg.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to compute package downloads")
		return
	}

	sender, err := h.queries.GetUser(r.Context(), pkg.CreatedByUserID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load package sender")
		return
	}

	resp := packagePreviewResponse{
		Name:               pkg.Name,
		SenderName:         sender.Name.String,
		Message:            pkg.Message,
		VerificationMethod: pkg.VerificationMethod,
		ExpiresAt:          pkg.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
		MaxDownloads:       maxDownloads,
		DownloadCount:      maxAccessCount,
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
				ShareID:     row.ShareID,
				Filename:    row.Filename,
				Size:        row.Size,
				AccessCount: row.AccessCount,
			}
		}
		resp.Files = files
	}

	writeJSON(w, http.StatusOK, resp)
}

type verifyPackageRequest struct {
	Password string `json:"password"`
}

// VerifyPackage takes a recipient's password and, on success, sets a
// per-package cookie holding an opaque DB-backed token — the same
// opaque-token-plus-row pattern as auth.SessionStore.
func (h *Handlers) VerifyPackage(w http.ResponseWriter, r *http.Request) {
	packageID := r.PathValue("id")

	pkg, err := h.queries.GetPackageByID(r.Context(), packageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, errPackageNotFound)
		return
	}

	if reason := unavailableReason(pkg, nil); reason != "" {
		writeJSONError(w, http.StatusGone, unavailableMessage(reason))
		return
	}

	if pkg.VerificationMethod != "password" {
		writeJSONError(w, http.StatusBadRequest, "Package does not use password verification")
		return
	}

	var req verifyPackageRequest
	if !decodePublicJSON(w, r, maxVerifyPackageBody, &req) {
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

// unavailableReason names why pkg is unreachable, or "" if it isn't.
// minAccessCount is the least-downloaded share's count (GetPackageMinAccessCount);
// nil skips the limit check, since the limit is per file, not per package.
func unavailableReason(pkg db.Package, minAccessCount *int64) string {
	switch {
	case pkg.Status != "active":
		return mail.ReasonRevoked
	case !pkg.ExpiresAt.After(time.Now()):
		return mail.ReasonExpired
	case pkg.MaxDownloads.Valid && minAccessCount != nil && *minAccessCount >= pkg.MaxDownloads.Int64:
		return mail.ReasonLimitReached
	default:
		return ""
	}
}

func unavailableMessage(reason string) string {
	switch reason {
	case mail.ReasonRevoked:
		return "This package has been revoked by the sender."
	case mail.ReasonExpired:
		return "This link has expired."
	case mail.ReasonLimitReached:
		return "Every file in this package has reached its download limit."
	default:
		return "This package is no longer available."
	}
}

// packageUnavailableResponse is the 410 body for a dead package. It carries the
// reason and sender so the page can offer to ask that person to reopen it —
// no more than the preview already reveals before verification.
type packageUnavailableResponse struct {
	Error            string `json:"error"`
	Reason           string `json:"reason"`
	Name             string `json:"name"`
	SenderName       string `json:"senderName"`
	CanRequestAccess bool   `json:"canRequestAccess"`
}

func (h *Handlers) writePackageUnavailable(w http.ResponseWriter, r *http.Request, pkg db.Package, reason string) {
	// Not fatal: the page falls back to "the sender".
	senderName := ""
	if sender, err := h.queries.GetUser(r.Context(), pkg.CreatedByUserID); err == nil {
		senderName = sender.Name.String
	}
	writeJSON(w, http.StatusGone, packageUnavailableResponse{
		Error:            unavailableMessage(reason),
		Reason:           reason,
		Name:             pkg.Name,
		SenderName:       senderName,
		CanRequestAccess: true,
	})
}
