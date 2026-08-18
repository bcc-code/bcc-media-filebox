package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	netmail "net/mail"
	"strings"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
)

// Throttles on this public endpoint: per email, and per package since a fresh
// address always passes the cooldown. vars so tests need not wait an hour.
var (
	accessRequestCooldown      = time.Hour
	accessRequestWindow        = time.Hour
	maxAccessRequestsPerWindow = 10
)

const (
	// Room for a real explanation, not for stuffing an inbox.
	maxAccessRequestMessage = 1000
	maxEmailLength          = 254
)

type requestPackageAccessRequest struct {
	Email   string `json:"email"`
	Message string `json:"message"`
}

type requestPackageAccessResponse struct {
	Requested  bool   `json:"requested"`
	SenderName string `json:"senderName"`
}

// RequestPackageAccess records a recipient's ask to reopen a dead package and
// mails its author. Unauthenticated on purpose — whoever needs it cannot get in
// — so anyone holding the link can post here: the address is taken at face value
// (it only becomes Reply-To, never a grant) and throttled per package AND email.
// Note that nothing caps requests per package: a fresh address always passes.
func (h *Handlers) RequestPackageAccess(w http.ResponseWriter, r *http.Request) {
	packageID := r.PathValue("id")

	pkg, err := h.queries.GetPackageByID(r.Context(), packageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, errPackageNotFound)
		return
	}

	var req requestPackageAccessRequest
	if !decodePublicJSON(w, r, maxAccessRequestBody, &req) {
		return
	}

	email := strings.TrimSpace(req.Email)
	if email == "" || len(email) > maxEmailLength {
		writeJSONError(w, http.StatusBadRequest, "A valid email address is required")
		return
	}
	// Take the bare address: ParseAddress also accepts "Name <addr>", and the
	// cooldown must see both forms as one.
	parsed, err := netmail.ParseAddress(email)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "A valid email address is required")
		return
	}
	email = parsed.Address

	message := strings.TrimSpace(req.Message)
	if len(message) > maxAccessRequestMessage {
		message = message[:maxAccessRequestMessage]
	}

	// Only a package that has actually stopped working can be requested. A
	// working link needs no request, and allowing one would turn every live
	// package into a channel for mailing its author.
	minAccessCount, err := h.queries.GetPackageMinAccessCount(r.Context(), pkg.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to check package state")
		return
	}
	reason := unavailableReason(pkg, &minAccessCount)
	if reason == "" {
		writeJSONError(w, http.StatusConflict, "This package is still available — try the download link again.")
		return
	}

	last, err := h.queries.GetLatestPackageAccessRequestByEmail(r.Context(), db.GetLatestPackageAccessRequestByEmailParams{
		PackageID: pkg.ID,
		Email:     email,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeJSONError(w, http.StatusInternalServerError, "Failed to check earlier requests")
		return
	}
	if err == nil && time.Since(last.CreatedAt) < accessRequestCooldown {
		writeJSONError(w, http.StatusTooManyRequests,
			"You've already asked about this package. The sender has your request — please wait for a reply.")
		return
	}

	recent, err := h.queries.CountRecentPackageAccessRequests(r.Context(), db.CountRecentPackageAccessRequestsParams{
		PackageID:     pkg.ID,
		WindowSeconds: int64(accessRequestWindow.Seconds()),
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to check earlier requests")
		return
	}
	if recent >= int64(maxAccessRequestsPerWindow) {
		writeJSONError(w, http.StatusTooManyRequests,
			"This package has had too many requests recently. The sender already has requests waiting — please try again later.")
		return
	}

	requestID, err := generateShareID()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to record request")
		return
	}

	created, err := h.queries.CreatePackageAccessRequest(r.Context(), db.CreatePackageAccessRequestParams{
		ID:        requestID,
		PackageID: pkg.ID,
		Email:     email,
		Message:   message,
		Reason:    reason,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to record request")
		return
	}

	// Detached, like notifyRecipients: the row is the durable record, and the
	// author sees it on the package card even if the mail never lands.
	go h.notifyAccessRequest(pkg, created, minAccessCount)

	senderName := ""
	if sender, err := h.queries.GetUser(r.Context(), pkg.CreatedByUserID); err == nil {
		senderName = sender.Name.String
	}

	writeJSON(w, http.StatusAccepted, requestPackageAccessResponse{Requested: true, SenderName: senderName})
}

type extendPackageRequest struct {
	// Counted from now, not from the old expiry.
	ExpiresInDays int `json:"expiresInDays"`
	// Replaces the stored per-file budget rather than adding to it; null means
	// unlimited, as in CreatePackageRequest.
	MaxDownloads *int `json:"maxDownloads"`
}

// ExtendPackage pushes a package's expiry out, resets its download budget, and
// un-revokes it. Author only. Every pending request is granted at once — they
// all want the same thing — and each requester is mailed the working link.
func (h *Handlers) ExtendPackage(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	packageID := r.PathValue("id")

	pkg, err := h.queries.GetPackageByID(r.Context(), packageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, errPackageNotFound)
		return
	}
	if pkg.CreatedByUserID != caller.UserID {
		writeJSONError(w, http.StatusForbidden, "not authorized to extend this package")
		return
	}

	var req extendPackageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.ExpiresInDays <= 0 || req.ExpiresInDays > maxShareExpiryDays {
		writeJSONError(w, http.StatusBadRequest,
			fmt.Sprintf("expiresInDays is required and must be between 1 and %d", maxShareExpiryDays))
		return
	}

	// Reject a non-positive budget rather than reading it as unlimited — that
	// would silently do the opposite of what the author asked for.
	if req.MaxDownloads != nil && *req.MaxDownloads < 1 {
		writeJSONError(w, http.StatusBadRequest, "maxDownloads must be at least 1, or omitted for unlimited")
		return
	}

	var maxDownloads sql.NullInt64
	if req.MaxDownloads != nil {
		maxDownloads = sql.NullInt64{Int64: int64(*req.MaxDownloads), Valid: true}
	}

	updated, err := h.queries.ExtendPackage(r.Context(), db.ExtendPackageParams{
		ExpiresAt:    time.Now().AddDate(0, 0, req.ExpiresInDays),
		MaxDownloads: maxDownloads,
		ID:           pkg.ID,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to extend package")
		return
	}

	granted, err := h.queries.GrantPendingPackageAccessRequests(r.Context(), pkg.ID)
	if err != nil {
		// The extension succeeded, so don't fail on this: the requests just stay
		// pending and the author can dismiss them.
		granted = nil
	}
	if len(granted) > 0 {
		go h.notifyAccessGranted(updated, granted, caller)
	}

	item, err := h.buildPackageListItem(r.Context(), updated)
	if err != nil {
		log.Printf("packages: reload after extending %s: %v", pkg.ID, err)
		writeJSONError(w, http.StatusInternalServerError, "failed to load the extended package")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// DismissPackageAccessRequest clears one request without granting it. Nothing
// is mailed — the request mail is already addressed to the requester, so an
// author saying no can just reply to it.
func (h *Handlers) DismissPackageAccessRequest(w http.ResponseWriter, r *http.Request) {
	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		writeJSONError(w, http.StatusForbidden, "authentication required")
		return
	}

	request, err := h.queries.GetPackageAccessRequest(r.Context(), r.PathValue("requestId"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, errAccessRequestNotFound)
		return
	}
	// Must match the package in the path too, so a valid id can't be aimed at
	// someone else's package.
	if request.PackageID != r.PathValue("id") {
		writeJSONError(w, http.StatusNotFound, errAccessRequestNotFound)
		return
	}

	pkg, err := h.queries.GetPackageByID(r.Context(), request.PackageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, errAccessRequestNotFound)
		return
	}
	if pkg.CreatedByUserID != caller.UserID {
		writeJSONError(w, http.StatusForbidden, "not authorized to dismiss this request")
		return
	}

	if err := h.queries.DismissPackageAccessRequest(r.Context(), request.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to dismiss request")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
