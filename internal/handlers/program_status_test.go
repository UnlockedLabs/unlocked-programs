package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/UnlockedLabs/unlocked-programs/internal/testutil"
	"github.com/stretchr/testify/require"
)

const programStatusPath = "/api/programs/%d/status"

func TestProgramStatusRejectsBadBodies(t *testing.T) {
	ts, client, srv, lms := newRouteTest(t)
	w := seedPrograms(t, srv.store, lms)
	admin := w.claims(w.facAdmin)

	bodies := map[string]any{
		"empty object":          map[string]any{},
		"is_active string":      map[string]any{"is_active": "false"},
		"is_active null":        map[string]any{"is_active": nil},
		"archived_at number":    map[string]any{"archived_at": 123},
		"archived_at null":      map[string]any{"archived_at": nil},
		"archived_at empty":     map[string]any{"archived_at": ""},
		"archived_at garbage":   map[string]any{"archived_at": "garbage"},
		"unknown key only":      map[string]any{"name": "x"},
		"array":                 []any{},
		"bare string":           "x",
		"archived_at bad type":  map[string]any{"archived_at": true, "is_active": true},
		"is_active number":      map[string]any{"is_active": 1},
		"archived_at date only": map[string]any{"archived_at": "2026-02-01"},
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			resp := call(t, ts, client, admin, http.MethodPatch, fmt.Sprintf(programStatusPath, 4), body)
			require.Equal(t, http.StatusBadRequest, resp.status, resp.body)
			require.NotEmpty(t, message(t, resp))
		})
	}

	var program models.Program
	require.NoError(t, srv.store.First(&program, 4).Error)
	require.True(t, program.IsActive)
	require.Nil(t, program.ArchivedAt)
}

func TestProgramStatusValidBodies(t *testing.T) {
	ts, client, srv, lms := newRouteTest(t)
	w := seedPrograms(t, srv.store, lms)
	admin := w.claims(w.facAdmin)
	patch := func(id int, body any) routeResponse {
		return call(t, ts, client, admin, http.MethodPatch, fmt.Sprintf(programStatusPath, id), body)
	}
	load := func(id int) models.Program {
		var p models.Program
		require.NoError(t, srv.store.First(&p, id).Error)
		return p
	}
	changes := func(id int) []models.ChangeLogEntry {
		var entries []models.ChangeLogEntry
		require.NoError(t, srv.store.Where("table_name = ? AND parent_ref_id = ?", "programs", id).Order("id").Find(&entries).Error)
		return entries
	}

	t.Run("archive writes one archived_at entry even with is_active", func(t *testing.T) {
		resp := patch(4, map[string]any{"archived_at": "2026-02-01T00:00:00Z", "is_active": false})
		require.Equal(t, http.StatusOK, resp.status, resp.body)
		p := load(4)
		require.NotNil(t, p.ArchivedAt)
		require.False(t, p.IsActive)
		entries := changes(4)
		require.Len(t, entries, 1)
		require.Equal(t, "archived_at", entries[0].FieldName)
	})

	t.Run("archived_at null with is_active clears the archive", func(t *testing.T) {
		resp := patch(3, map[string]any{"archived_at": nil, "is_active": true})
		require.Equal(t, http.StatusOK, resp.status, resp.body)
		p := load(3)
		require.Nil(t, p.ArchivedAt)
		require.True(t, p.IsActive)
		entries := changes(3)
		require.Len(t, entries, 1)
		require.Equal(t, "is_active", entries[0].FieldName)
	})

	t.Run("is_active alone leaves archived_at untouched", func(t *testing.T) {
		resp := patch(4, map[string]any{"is_active": true})
		require.Equal(t, http.StatusOK, resp.status, resp.body)
		p := load(4)
		require.NotNil(t, p.ArchivedAt)
		require.True(t, p.IsActive)
	})

	t.Run("unknown keys are ignored", func(t *testing.T) {
		resp := patch(2, map[string]any{"is_active": false, "name": "Renamed", "description": "changed"})
		require.Equal(t, http.StatusOK, resp.status, resp.body)
		p := load(2)
		require.Equal(t, "Welding", p.Name)
		require.Equal(t, "Shop safety and welding", p.Description)
		require.False(t, p.IsActive)
	})
}

func TestProgramStatusMalformedJSONKeepsGenericMessage(t *testing.T) {
	ts, client, srv, lms := newRouteTest(t)
	w := seedPrograms(t, srv.store, lms)
	claims, err := json.Marshal(w.claims(w.facAdmin))
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch, ts.URL+fmt.Sprintf(programStatusPath, 4), strings.NewReader(`{"is_active":`))
	require.NoError(t, err)
	req.Header.Set(testutil.ClaimsHeader, string(claims))
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "error reading json request", body.Message)
}
