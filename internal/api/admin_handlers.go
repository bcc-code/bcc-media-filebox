package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"filebox/internal/auth"
	db "filebox/internal/db/gen"
	"filebox/internal/forms"
)

// AdminHandlers exposes /api/admin/* endpoints, all gated by users.role=='admin'.
type AdminHandlers struct {
	queries *db.Queries
}

func NewAdminHandlers(queries *db.Queries) *AdminHandlers {
	return &AdminHandlers{queries: queries}
}

// Register wires every admin route under /api/admin/ behind the requireAdmin
// middleware. Non-admins get a JSON 403; unauthenticated guests too.
func (h *AdminHandlers) Register(mux *http.ServeMux) {
	wrap := func(fn http.HandlerFunc) http.HandlerFunc { return requireAdmin(fn) }

	mux.HandleFunc("GET /api/admin/targets", wrap(h.ListTargets))
	mux.HandleFunc("POST /api/admin/targets", wrap(h.CreateTarget))
	mux.HandleFunc("POST /api/admin/targets/reorder", wrap(h.ReorderTargets))
	mux.HandleFunc("PATCH /api/admin/targets/{id}", wrap(h.UpdateTarget))
	mux.HandleFunc("DELETE /api/admin/targets/{id}", wrap(h.DeleteTarget))

	mux.HandleFunc("GET /api/admin/projects", wrap(h.ListProjects))
	mux.HandleFunc("POST /api/admin/projects", wrap(h.CreateProject))
	mux.HandleFunc("PATCH /api/admin/projects/{id}", wrap(h.UpdateProject))
	mux.HandleFunc("DELETE /api/admin/projects/{id}", wrap(h.DeleteProject))

	mux.HandleFunc("GET /api/admin/users", wrap(h.ListUsers))
	mux.HandleFunc("GET /api/admin/users/{id}", wrap(h.GetUser))

	mux.HandleFunc("GET /api/admin/groups", wrap(h.ListGroups))
	mux.HandleFunc("POST /api/admin/groups", wrap(h.CreateGroup))
	mux.HandleFunc("PATCH /api/admin/groups/{id}", wrap(h.UpdateGroup))
	mux.HandleFunc("DELETE /api/admin/groups/{id}", wrap(h.DeleteGroup))

	mux.HandleFunc("GET /api/admin/grants", wrap(h.ListGrants))
	mux.HandleFunc("POST /api/admin/grants", wrap(h.CreateGrant))
	mux.HandleFunc("PATCH /api/admin/grants/{id}", wrap(h.UpdateGrant))
	mux.HandleFunc("DELETE /api/admin/grants/{id}", wrap(h.DeleteGrant))
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller := auth.CallerFrom(r.Context())
		if caller == nil || caller.Role != "admin" {
			writeJSONError(w, http.StatusForbidden, "admin role required")
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseID(r *http.Request) (int64, error) {
	raw := r.PathValue("id")
	return strconv.ParseInt(raw, 10, 64)
}

// ---------- Targets ----------

type targetDTO struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Path      string  `json:"path"`
	FormKey   *string `json:"formKey"`
	Position  int64   `json:"position"`
	CreatedAt string  `json:"createdAt"`
}

func targetToDTO(t db.Target) targetDTO {
	dto := targetDTO{
		ID:        t.ID,
		Name:      t.Name,
		Path:      t.Path,
		Position:  t.Position,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
	if t.FormKey.Valid && t.FormKey.String != "" {
		dto.FormKey = &t.FormKey.String
	}
	return dto
}

func (h *AdminHandlers) ListTargets(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListTargets(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]targetDTO, len(rows))
	for i, t := range rows {
		out[i] = targetToDTO(t)
	}
	writeJSON(w, http.StatusOK, out)
}

type targetWriteRequest struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	FormKey *string `json:"formKey"`
}

// formKeyParam normalizes the request's form key into the nullable column value,
// treating an empty string the same as "no form".
func (req targetWriteRequest) formKeyParam() sql.NullString {
	if req.FormKey == nil || strings.TrimSpace(*req.FormKey) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: strings.TrimSpace(*req.FormKey), Valid: true}
}

func (h *AdminHandlers) CreateTarget(w http.ResponseWriter, r *http.Request) {
	var req targetWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	if err := validateTarget(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	t, err := h.queries.CreateTarget(r.Context(), db.CreateTargetParams{Name: req.Name, Path: req.Path, FormKey: req.formKeyParam()})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, targetToDTO(t))
}

func (h *AdminHandlers) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req targetWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	if err := validateTarget(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	t, err := h.queries.UpdateTarget(r.Context(), db.UpdateTargetParams{Name: req.Name, Path: req.Path, FormKey: req.formKeyParam(), ID: id})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, targetToDTO(t))
}

type reorderTargetsRequest struct {
	IDs []int64 `json:"ids"`
}

// ReorderTargets persists a new global ordering for targets. The request body
// must contain the full id set in the desired order — any missing, extra, or
// duplicate id is rejected with 400. Positions are renumbered 1..N.
func (h *AdminHandlers) ReorderTargets(w http.ResponseWriter, r *http.Request) {
	var req reorderTargetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	existing, err := h.queries.ListTargets(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(req.IDs) != len(existing) {
		writeJSONError(w, http.StatusBadRequest, "ids must match the existing target set")
		return
	}
	have := make(map[int64]struct{}, len(existing))
	for _, t := range existing {
		have[t.ID] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(req.IDs))
	for _, id := range req.IDs {
		if _, dup := seen[id]; dup {
			writeJSONError(w, http.StatusBadRequest, "duplicate id in request")
			return
		}
		if _, ok := have[id]; !ok {
			writeJSONError(w, http.StatusBadRequest, "unknown target id in request")
			return
		}
		seen[id] = struct{}{}
	}
	for i, id := range req.IDs {
		if err := h.queries.UpdateTargetPosition(r.Context(), db.UpdateTargetPositionParams{
			Position: int64(i + 1),
			ID:       id,
		}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	rows, err := h.queries.ListTargets(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]targetDTO, len(rows))
	for i, t := range rows {
		out[i] = targetToDTO(t)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AdminHandlers) DeleteTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	// grant_targets has ON DELETE CASCADE, so any grants referencing this
	// target will lose just the target reference (not the whole grant).
	if err := h.queries.DeleteTarget(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateTarget(req targetWriteRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Path == "" {
		return errors.New("path is required")
	}
	info, err := os.Stat(req.Path)
	if err != nil {
		return fmt.Errorf("path %q: %w", req.Path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", req.Path)
	}
	if req.FormKey != nil && strings.TrimSpace(*req.FormKey) != "" {
		if _, ok := forms.Get(strings.TrimSpace(*req.FormKey)); !ok {
			return fmt.Errorf("unknown form %q", *req.FormKey)
		}
	}
	return nil
}

// ---------- Projects ----------

// projectCodePattern restricts codes to filename-safe characters since the code
// is embedded verbatim in derived filenames.
var projectCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type projectDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	CreatedAt string `json:"createdAt"`
}

func projectToDTO(p db.Project) projectDTO {
	return projectDTO{
		ID:        p.ID,
		Name:      p.Name,
		Code:      p.Code,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
	}
}

type projectWriteRequest struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

func validateProject(req projectWriteRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Code == "" {
		return errors.New("code is required")
	}
	if !projectCodePattern.MatchString(req.Code) {
		return errors.New("code may contain only letters, digits, '-' and '_'")
	}
	return nil
}

func (h *AdminHandlers) ListProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListProjects(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]projectDTO, len(rows))
	for i, p := range rows {
		out[i] = projectToDTO(p)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AdminHandlers) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req projectWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if err := validateProject(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := h.queries.CreateProject(r.Context(), db.CreateProjectParams{Name: req.Name, Code: req.Code})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, projectToDTO(p))
}

func (h *AdminHandlers) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req projectWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if err := validateProject(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := h.queries.UpdateProject(r.Context(), db.UpdateProjectParams{Name: req.Name, Code: req.Code, ID: id})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projectToDTO(p))
}

func (h *AdminHandlers) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.queries.DeleteProject(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Users ----------

type userListDTO struct {
	ID               int64   `json:"id"`
	Provider         string  `json:"provider"`
	Email            string  `json:"email"`
	Name             string  `json:"name"`
	Role             string  `json:"role"`
	CreatedAt        string  `json:"createdAt"`
	LastLoginAt      string  `json:"lastLoginAt"`
	Uploads          int64   `json:"uploads"`
	UploadsThisMonth int64   `json:"uploadsThisMonth"`
	TotalBytes       int64   `json:"totalBytes"`
	BytesThisMonth   int64   `json:"bytesThisMonth"`
	Failures         int64   `json:"failures"`
	Active           bool    `json:"active"`
	Groups           []string `json:"groups"`
}

func userCanonicalUserID(u db.User) string {
	if u.Provider == "guest" {
		return "guest:" + u.Subject
	}
	return u.Provider + ":" + u.Subject
}

func (h *AdminHandlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListUsers(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]userListDTO, 0, len(users))
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	for _, u := range users {
		stats, err := h.queries.UserUploadStats(r.Context(), userCanonicalUserID(u))
		if err != nil {
			log.Printf("admin: user stats for %d: %v", u.ID, err)
			continue
		}
		dto := userListDTO{
			ID:               u.ID,
			Provider:         u.Provider,
			Email:            u.Email.String,
			Name:             u.Name.String,
			Role:             u.Role,
			CreatedAt:        u.CreatedAt.Format(time.RFC3339),
			LastLoginAt:      u.LastLoginAt.Format(time.RFC3339),
			Uploads:          coalesceInt64(stats.Uploads),
			UploadsThisMonth: coalesceInt64(stats.UploadsThisMonth),
			TotalBytes:       coalesceInt64(stats.TotalBytes),
			BytesThisMonth:   coalesceInt64(stats.BytesThisMonth),
			Failures:         coalesceInt64(stats.Failures),
			Active:           u.LastLoginAt.After(thirtyDaysAgo),
			Groups:           []string{},
		}
		if u.Email.Valid && u.Email.String != "" {
			names, err := h.queries.ListGroupNamesForEmail(r.Context(), db.ListGroupNamesForEmailParams{
				Email:    u.Email.String,
				Provider: u.Provider,
			})
			if err == nil {
				dto.Groups = names
			}
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

// userDetailDTO extends the list DTO with everything the drawer needs.
type userDetailDTO struct {
	userListDTO
	Recent            []recentUploadDTO `json:"recent"`
	DirectGrants      []grantDTO        `json:"directGrants"`
	EffectiveTargetIDs []int64           `json:"effectiveTargetIds"`
	EffectiveAll      bool              `json:"effectiveAll"`
}

type recentUploadDTO struct {
	ID         string `json:"id"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	TargetName string `json:"targetName"`
	When       string `json:"when"`
}

func (h *AdminHandlers) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	u, err := h.queries.GetUser(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "user not found")
		return
	}
	stats, err := h.queries.UserUploadStats(r.Context(), userCanonicalUserID(u))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recentRows, err := h.queries.UserRecentUploads(r.Context(), userCanonicalUserID(u))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recent := make([]recentUploadDTO, len(recentRows))
	for i, ru := range recentRows {
		when := ru.CreatedAt
		if ru.CompletedAt.Valid {
			when = ru.CompletedAt.Time
		}
		recent[i] = recentUploadDTO{
			ID:         ru.ID,
			Filename:   ru.Filename,
			Size:       ru.Size,
			TargetName: ru.TargetName.String,
			When:       when.Format(time.RFC3339),
		}
	}

	groupNames := []string{}
	directGrantRows := []db.GrantsForUserRow{}
	allowed := AllowedTargets{IDs: map[int64]struct{}{}}
	if u.Email.Valid && u.Email.String != "" {
		gnames, err := h.queries.ListGroupNamesForEmail(r.Context(), db.ListGroupNamesForEmailParams{
			Email:    u.Email.String,
			Provider: u.Provider,
		})
		if err == nil {
			groupNames = gnames
		}
		rows, err := h.queries.GrantsForUser(r.Context(), db.GrantsForUserParams{
			Email:    u.Email.String,
			Provider: u.Provider,
		})
		if err == nil {
			for _, g := range rows {
				if g.PrincipalKind == "user" {
					directGrantRows = append(directGrantRows, g)
				}
				if g.Admin != 0 {
					allowed.All = true
				}
				if g.AllTargets != 0 {
					allowed.All = true
				}
				for _, idStr := range splitTargetIDs(g.TargetIds) {
					if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
						allowed.IDs[id] = struct{}{}
					}
				}
			}
		}
	}
	if u.Role == "admin" {
		allowed.All = true
	}

	directGrants := make([]grantDTO, len(directGrantRows))
	for i, g := range directGrantRows {
		directGrants[i] = grantRowToDTO(g)
	}

	effective := []int64{}
	if !allowed.All {
		for id := range allowed.IDs {
			effective = append(effective, id)
		}
	}

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	listDTO := userListDTO{
		ID:               u.ID,
		Provider:         u.Provider,
		Email:            u.Email.String,
		Name:             u.Name.String,
		Role:             u.Role,
		CreatedAt:        u.CreatedAt.Format(time.RFC3339),
		LastLoginAt:      u.LastLoginAt.Format(time.RFC3339),
		Uploads:          coalesceInt64(stats.Uploads),
		UploadsThisMonth: coalesceInt64(stats.UploadsThisMonth),
		TotalBytes:       coalesceInt64(stats.TotalBytes),
		BytesThisMonth:   coalesceInt64(stats.BytesThisMonth),
		Failures:         coalesceInt64(stats.Failures),
		Active:           u.LastLoginAt.After(thirtyDaysAgo),
		Groups:           groupNames,
	}
	writeJSON(w, http.StatusOK, userDetailDTO{
		userListDTO:        listDTO,
		Recent:             recent,
		DirectGrants:       directGrants,
		EffectiveTargetIDs: effective,
		EffectiveAll:       allowed.All,
	})
}

func coalesceInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case []byte:
		n, _ := strconv.ParseInt(string(x), 10, 64)
		return n
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	case nil:
		return 0
	default:
		return 0
	}
}

// ---------- Groups ----------

type groupDTO struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	CreatedAt   string   `json:"createdAt"`
	MemberCount int64    `json:"memberCount"`
	Members     []string `json:"members"`
}

func (h *AdminHandlers) ListGroups(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListGroups(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]groupDTO, len(rows))
	for i, gr := range rows {
		members := []string{}
		if gr.Kind == "custom" {
			ms, err := h.queries.ListGroupMembers(r.Context(), gr.ID)
			if err == nil {
				members = ms
			}
		}
		out[i] = groupDTO{
			ID:          gr.ID,
			Name:        gr.Name,
			Kind:        gr.Kind,
			Description: gr.Description,
			CreatedAt:   gr.CreatedAt.Format(time.RFC3339),
			MemberCount: gr.MemberCount,
			Members:     members,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type groupWriteRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

func (h *AdminHandlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req groupWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}
	gr, err := h.queries.CreateGroup(r.Context(), db.CreateGroupParams{
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.replaceGroupMembers(r.Context(), gr.ID, req.Members); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = RecomputeUserRoles(r.Context(), h.queries)
	h.writeGroupDTO(w, http.StatusCreated, gr.ID)
}

func (h *AdminHandlers) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req groupWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Need the old name so we can keep referencing grants in sync if it changed.
	old, err := h.queries.GetGroup(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "group not found")
		return
	}
	if old.Kind != "custom" {
		writeJSONError(w, http.StatusBadRequest, "built-in groups are read-only")
		return
	}

	gr, err := h.queries.UpdateGroup(r.Context(), db.UpdateGroupParams{
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
		ID:          id,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if old.Name != gr.Name {
		// Rename grants that reference the old group name.
		if _, err := renameGroupGrants(r.Context(), h.queries, old.Name, gr.Name); err != nil {
			log.Printf("rename group grants: %v", err)
		}
	}
	if err := h.replaceGroupMembers(r.Context(), gr.ID, req.Members); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = RecomputeUserRoles(r.Context(), h.queries)
	h.writeGroupDTO(w, http.StatusOK, gr.ID)
}

func (h *AdminHandlers) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	old, err := h.queries.GetGroup(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "group not found")
		return
	}
	if old.Kind != "custom" {
		writeJSONError(w, http.StatusBadRequest, "built-in groups are read-only")
		return
	}
	// Cascade-delete any grants referencing this group by name.
	if err := h.queries.DeleteGrantsByGroupName(r.Context(), old.Name); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.queries.DeleteGroup(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = RecomputeUserRoles(r.Context(), h.queries)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandlers) replaceGroupMembers(ctx context.Context, groupID int64, members []string) error {
	if err := h.queries.ClearGroupMembers(ctx, groupID); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, m := range members {
		m = strings.ToLower(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		if err := h.queries.AddGroupMember(ctx, db.AddGroupMemberParams{GroupID: groupID, Email: m}); err != nil {
			return err
		}
	}
	return nil
}

func (h *AdminHandlers) writeGroupDTO(w http.ResponseWriter, status int, id int64) {
	gr, err := h.queries.GetGroup(context.Background(), id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	members, err := h.queries.ListGroupMembers(context.Background(), id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, status, groupDTO{
		ID:          gr.ID,
		Name:        gr.Name,
		Kind:        gr.Kind,
		Description: gr.Description,
		CreatedAt:   gr.CreatedAt.Format(time.RFC3339),
		MemberCount: int64(len(members)),
		Members:     members,
	})
}

func renameGroupGrants(ctx context.Context, queries *db.Queries, oldName, newName string) (int64, error) {
	// No bulk-update query — load + write the ones that match.
	rows, err := queries.ListGrants(ctx)
	if err != nil {
		return 0, err
	}
	updated := int64(0)
	for _, g := range rows {
		if g.PrincipalKind == "group" && g.PrincipalValue == oldName {
			if err := queries.UpdateGrantPrincipal(ctx, db.UpdateGrantPrincipalParams{
				PrincipalKind:  "group",
				PrincipalValue: newName,
				ID:             g.ID,
			}); err != nil {
				return updated, err
			}
			updated++
		}
	}
	return updated, nil
}

// ---------- Grants ----------

type grantDTO struct {
	ID             int64   `json:"id"`
	PrincipalKind  string  `json:"principalKind"`
	PrincipalValue string  `json:"principalValue"`
	Admin          bool    `json:"admin"`
	AllTargets     bool    `json:"allTargets"`
	TargetIDs      []int64 `json:"targetIds"`
	CreatedAt      string  `json:"createdAt"`
}

func grantRowToDTO(g db.GrantsForUserRow) grantDTO {
	ids := []int64{}
	for _, s := range splitTargetIDs(g.TargetIds) {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			ids = append(ids, n)
		}
	}
	return grantDTO{
		ID:             g.ID,
		PrincipalKind:  g.PrincipalKind,
		PrincipalValue: g.PrincipalValue,
		Admin:          g.Admin != 0,
		AllTargets:     g.AllTargets != 0,
		TargetIDs:      ids,
		CreatedAt:      g.CreatedAt.Format(time.RFC3339),
	}
}

func listGrantRowToDTO(g db.ListGrantsRow) grantDTO {
	ids := []int64{}
	for _, s := range splitTargetIDs(g.TargetIds) {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			ids = append(ids, n)
		}
	}
	return grantDTO{
		ID:             g.ID,
		PrincipalKind:  g.PrincipalKind,
		PrincipalValue: g.PrincipalValue,
		Admin:          g.Admin != 0,
		AllTargets:     g.AllTargets != 0,
		TargetIDs:      ids,
		CreatedAt:      g.CreatedAt.Format(time.RFC3339),
	}
}

func getGrantRowToDTO(g db.GetGrantRow) grantDTO {
	ids := []int64{}
	for _, s := range splitTargetIDs(g.TargetIds) {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			ids = append(ids, n)
		}
	}
	return grantDTO{
		ID:             g.ID,
		PrincipalKind:  g.PrincipalKind,
		PrincipalValue: g.PrincipalValue,
		Admin:          g.Admin != 0,
		AllTargets:     g.AllTargets != 0,
		TargetIDs:      ids,
		CreatedAt:      g.CreatedAt.Format(time.RFC3339),
	}
}

func (h *AdminHandlers) ListGrants(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListGrants(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]grantDTO, len(rows))
	for i, g := range rows {
		out[i] = listGrantRowToDTO(g)
	}
	writeJSON(w, http.StatusOK, out)
}

type grantWriteRequest struct {
	PrincipalKind  string  `json:"principalKind"`
	PrincipalValue string  `json:"principalValue"`
	Admin          bool    `json:"admin"`
	AllTargets     bool    `json:"allTargets"`
	TargetIDs      []int64 `json:"targetIds"`
}

func (req grantWriteRequest) validate() error {
	if req.PrincipalKind != "user" && req.PrincipalKind != "group" {
		return errors.New("principalKind must be 'user' or 'group'")
	}
	value := strings.TrimSpace(req.PrincipalValue)
	if value == "" {
		return errors.New("principalValue is required")
	}
	// Guests can never be admins. Block the obvious misconfiguration at
	// write time; computeRoleFor enforces the same rule defensively at
	// role-resolution time.
	if req.Admin && req.PrincipalKind == "group" && value == "All guests" {
		return errors.New("the 'All guests' group cannot be granted admin")
	}
	return nil
}

func (h *AdminHandlers) CreateGrant(w http.ResponseWriter, r *http.Request) {
	var req grantWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := req.validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	admin := int64(0)
	if req.Admin {
		admin = 1
	}
	all := int64(0)
	if req.AllTargets {
		all = 1
	}
	value := strings.TrimSpace(req.PrincipalValue)
	if req.PrincipalKind == "user" {
		value = strings.ToLower(value)
	}
	g, err := h.queries.CreateGrant(r.Context(), db.CreateGrantParams{
		PrincipalKind:  req.PrincipalKind,
		PrincipalValue: value,
		Admin:          admin,
		AllTargets:     all,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !req.Admin && !req.AllTargets {
		if err := h.replaceGrantTargets(r.Context(), g.ID, req.TargetIDs); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	_ = RecomputeUserRoles(r.Context(), h.queries)
	h.writeGrantDTO(w, http.StatusCreated, g.ID)
}

func (h *AdminHandlers) UpdateGrant(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req grantWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := req.validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	admin := int64(0)
	if req.Admin {
		admin = 1
	}
	all := int64(0)
	if req.AllTargets {
		all = 1
	}
	value := strings.TrimSpace(req.PrincipalValue)
	if req.PrincipalKind == "user" {
		value = strings.ToLower(value)
	}
	if err := h.queries.UpdateGrantPrincipal(r.Context(), db.UpdateGrantPrincipalParams{
		PrincipalKind:  req.PrincipalKind,
		PrincipalValue: value,
		ID:             id,
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.queries.UpdateGrant(r.Context(), db.UpdateGrantParams{Admin: admin, AllTargets: all, ID: id}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Admin || req.AllTargets {
		_ = h.queries.ClearGrantTargets(r.Context(), id)
	} else {
		if err := h.replaceGrantTargets(r.Context(), id, req.TargetIDs); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	_ = RecomputeUserRoles(r.Context(), h.queries)
	h.writeGrantDTO(w, http.StatusOK, id)
}

func (h *AdminHandlers) DeleteGrant(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.queries.DeleteGrant(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = RecomputeUserRoles(r.Context(), h.queries)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandlers) replaceGrantTargets(ctx context.Context, grantID int64, ids []int64) error {
	if err := h.queries.ClearGrantTargets(ctx, grantID); err != nil {
		return err
	}
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if err := h.queries.AddGrantTarget(ctx, db.AddGrantTargetParams{GrantID: grantID, TargetID: id}); err != nil {
			return err
		}
	}
	return nil
}

func (h *AdminHandlers) writeGrantDTO(w http.ResponseWriter, status int, id int64) {
	row, err := h.queries.GetGrant(context.Background(), id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, status, getGrantRowToDTO(row))
}

// unused — kept to satisfy import in case of future use
var _ = sql.NullString{}
var _ = auth.SessionCookieName
