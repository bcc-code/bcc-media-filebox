package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
)

type Handlers struct {
	queries *db.Queries
}

func NewHandlers(queries *db.Queries) *Handlers {
	return &Handlers{queries: queries}
}

type UploadResponse struct {
	ID           string   `json:"id"`
	Filename     string   `json:"filename"`
	Size         int64    `json:"size"`
	Offset       int64    `json:"offset"`
	ContentType  *string  `json:"contentType"`
	Status       string   `json:"status"`
	DurationMs   *int64   `json:"durationMs"`
	AvgBandwidth *float64 `json:"avgBandwidth"`
	SHA256       *string  `json:"sha256"`
	CreatedAt    string   `json:"createdAt"`
	CompletedAt  *string  `json:"completedAt"`
}

func toResponse(u db.Upload) UploadResponse {
	r := UploadResponse{
		ID:        u.ID,
		Filename:  u.Filename,
		Size:      u.Size,
		Offset:    u.Offset,
		Status:    u.Status,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if u.ContentType.Valid {
		r.ContentType = &u.ContentType.String
	}
	if u.Sha256.Valid {
		r.SHA256 = &u.Sha256.String
	}
	if u.CompletedAt.Valid {
		r.CompletedAt = new(u.CompletedAt.Time.Format("2006-01-02T15:04:05Z"))
	}
	if u.DurationMs.Valid && u.DurationMs.Int64 > 0 {
		r.DurationMs = &u.DurationMs.Int64
		// Average bandwidth in bytes/sec
		r.AvgBandwidth = new(float64(u.Size) / (float64(u.DurationMs.Int64) / 1000.0))
	}
	return r
}

// targetView is the user-facing shape of a target: the friendly name plus the
// optional hardcoded form key the uploader must fill before uploading.
type targetView struct {
	Name    string  `json:"name"`
	FormKey *string `json:"formKey"`
}

func targetToView(t db.Target) targetView {
	v := targetView{Name: t.Name}
	if t.FormKey.Valid && t.FormKey.String != "" {
		v.FormKey = &t.FormKey.String
	}
	return v
}

// ListTargets returns the upload targets the caller is allowed to write to.
// Authenticated callers are filtered by the grants table:
//   - role=admin or any grant with all_targets=1 → all targets
//   - otherwise → union of target_ids across all matching grants
//
// Guests fall through unfiltered (no grants concept for them yet); empty list
// for an authenticated non-admin with no grants is the correct "permission wall"
// state described in the design.
func (h *Handlers) ListTargets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	all, err := h.queries.ListTargets(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	caller := auth.CallerFrom(r.Context())
	if caller == nil {
		// Unauthenticated — the LoginGate prevents the UI from rendering this
		// state, but return everything so the picker isn't empty in dev.
		views := make([]targetView, len(all))
		for i, t := range all {
			views[i] = targetToView(t)
		}
		_ = json.NewEncoder(w).Encode(views)
		return
	}

	allowed, err := EffectiveTargetIDs(r.Context(), h.queries, caller)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if allowed.All {
		views := make([]targetView, len(all))
		for i, t := range all {
			views[i] = targetToView(t)
		}
		_ = json.NewEncoder(w).Encode(views)
		return
	}

	views := make([]targetView, 0, len(allowed.IDs))
	for _, t := range all {
		if _, ok := allowed.IDs[t.ID]; ok {
			views = append(views, targetToView(t))
		}
	}
	_ = json.NewEncoder(w).Encode(views)
}

// projectView is the user-facing shape of a project for form dropdowns: the
// visible name plus the code that gets embedded in the derived filename.
type projectView struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// ListProjects returns all projects for population of form select fields whose
// optionsSource is "projects".
func (h *Handlers) ListProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rows, err := h.queries.ListProjects(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]projectView, len(rows))
	for i, p := range rows {
		out[i] = projectView{Name: p.Name, Code: p.Code}
	}
	_ = json.NewEncoder(w).Encode(out)
}

// ProjectSuggestions returns the distinct season and episode values previously
// used for a project (by code), powering the free-text autocomplete in forms.
func (h *Handlers) ProjectSuggestions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	code := sql.NullString{String: r.PathValue("code"), Valid: true}

	seasons, err := h.queries.ProjectSeasons(r.Context(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	episodes, err := h.queries.ProjectEpisodes(r.Context(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if seasons == nil {
		seasons = []string{}
	}
	if episodes == nil {
		episodes = []string{}
	}
	_ = json.NewEncoder(w).Encode(map[string][]string{"seasons": seasons, "episodes": episodes})
}

// nameCodeView is the user-facing shape for catalog dropdowns (arrangements,
// sub-events): visible name + the code embedded in the filename.
type nameCodeView struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// ListArrangements returns all arrangements for the Arrangement select field.
func (h *Handlers) ListArrangements(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rows, err := h.queries.ListArrangements(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]nameCodeView, len(rows))
	for i, a := range rows {
		out[i] = nameCodeView{Name: a.Name, Code: a.Code}
	}
	_ = json.NewEncoder(w).Encode(out)
}

// ListSubEvents returns the sub-events belonging to one arrangement (by code),
// powering the dependent Sub event dropdown.
func (h *Handlers) ListSubEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rows, err := h.queries.ListSubEventsByArrangementCode(r.Context(), r.PathValue("code"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]nameCodeView, len(rows))
	for i, s := range rows {
		out[i] = nameCodeView{Name: s.Name, Code: s.Code}
	}
	_ = json.NewEncoder(w).Encode(out)
}

// ListUploads returns the history for the calling user. When the request is
// authenticated, the session's canonical user_id is authoritative and the
// user_id query parameter is ignored. Guests must supply user_id, but cannot
// peek at authenticated users' histories: any value containing a ":" that
// isn't a "guest:" id is rejected. Legacy raw-ULID ids (no colon) remain
// queryable so pre-OAuth uploads stay accessible.
func (h *Handlers) ListUploads(w http.ResponseWriter, r *http.Request) {
	var userID string
	if caller := auth.CallerFrom(r.Context()); caller != nil {
		userID = caller.CanonicalUserID()
	} else {
		userID = r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "missing user_id parameter", http.StatusBadRequest)
			return
		}
		if strings.Contains(userID, ":") && !strings.HasPrefix(userID, "guest:") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	uploads, err := h.queries.ListUploads(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]UploadResponse, len(uploads))
	for i, u := range uploads {
		result[i] = toResponse(u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
